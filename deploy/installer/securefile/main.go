//go:build linux

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const noSymlinkMessage = "sensitive path must not contain symbolic links or .. components"

type fileIdentity struct {
	device     uint64
	inode      uint64
	size       int64
	modifiedNS int64
	changedNS  int64
}

func identityFromStat(stat *syscall.Stat_t) fileIdentity {
	return fileIdentity{
		device:     uint64(stat.Dev),
		inode:      stat.Ino,
		size:       stat.Size,
		modifiedNS: stat.Mtim.Sec*int64(time.Second) + stat.Mtim.Nsec,
		changedNS:  stat.Ctim.Sec*int64(time.Second) + stat.Ctim.Nsec,
	}
}

func (identity fileIdentity) String() string {
	return fmt.Sprintf("%d:%d:%d:%d:%d", identity.device, identity.inode, identity.size, identity.modifiedNS, identity.changedNS)
}

func parseIdentity(value string) (fileIdentity, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 5 {
		return fileIdentity{}, errors.New("invalid expected file identity")
	}
	values := make([]int64, len(parts))
	for index, part := range parts {
		parsed, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return fileIdentity{}, errors.New("invalid expected file identity")
		}
		values[index] = parsed
	}
	return fileIdentity{
		device:     uint64(values[0]),
		inode:      uint64(values[1]),
		size:       values[2],
		modifiedNS: values[3],
		changedNS:  values[4],
	}, nil
}

func secureComponents(path string) ([]string, error) {
	if path == "" {
		return nil, errors.New("sensitive path is required")
	}
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		if component == ".." {
			return nil, errors.New(noSymlinkMessage)
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve sensitive path: %w", err)
	}
	components := strings.Split(strings.TrimPrefix(filepath.Clean(absolute), string(filepath.Separator)), string(filepath.Separator))
	if len(components) == 0 || components[len(components)-1] == "" {
		return nil, errors.New("sensitive path must name a file")
	}
	return components, nil
}

func openParent(path string, createParents bool) (int, string, error) {
	components, err := secureComponents(path)
	if err != nil {
		return -1, "", err
	}
	directoryFD, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return -1, "", fmt.Errorf("open sensitive path root: %w", err)
	}
	for _, component := range components[:len(components)-1] {
		nextFD, openErr := syscall.Openat(directoryFD, component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if errors.Is(openErr, syscall.ENOENT) && createParents {
			if mkdirErr := syscall.Mkdirat(directoryFD, component, 0700); mkdirErr != nil && !errors.Is(mkdirErr, syscall.EEXIST) {
				_ = syscall.Close(directoryFD)
				return -1, "", fmt.Errorf("create sensitive path parent: %w", mkdirErr)
			}
			nextFD, openErr = syscall.Openat(directoryFD, component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		}
		if openErr != nil {
			_ = syscall.Close(directoryFD)
			return -1, "", errors.New(noSymlinkMessage)
		}
		_ = syscall.Close(directoryFD)
		directoryFD = nextFD
	}
	return directoryFD, components[len(components)-1], nil
}

func openRegularAt(directoryFD int, name string, flags int) (int, fileIdentity, error) {
	fileFD, err := syscall.Openat(directoryFD, name, flags|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return -1, fileIdentity{}, errors.New(noSymlinkMessage)
	}
	var stat syscall.Stat_t
	if err := syscall.Fstat(fileFD, &stat); err != nil {
		_ = syscall.Close(fileFD)
		return -1, fileIdentity{}, fmt.Errorf("inspect sensitive file: %w", err)
	}
	if stat.Mode&syscall.S_IFMT != syscall.S_IFREG {
		_ = syscall.Close(fileFD)
		return -1, fileIdentity{}, errors.New("sensitive file must be regular")
	}
	return fileFD, identityFromStat(&stat), nil
}

func copyFD(destinationFD int, sourceFD int) error {
	destinationCopy, err := syscall.Dup(destinationFD)
	if err != nil {
		return fmt.Errorf("duplicate destination file descriptor: %w", err)
	}
	sourceCopy, err := syscall.Dup(sourceFD)
	if err != nil {
		_ = syscall.Close(destinationCopy)
		return fmt.Errorf("duplicate source file descriptor: %w", err)
	}
	destination := os.NewFile(uintptr(destinationCopy), "destination")
	source := os.NewFile(uintptr(sourceCopy), "source")
	if destination == nil || source == nil {
		_ = syscall.Close(destinationCopy)
		_ = syscall.Close(sourceCopy)
		return errors.New("create secure file handle")
	}
	defer destination.Close()
	defer source.Close()
	_, err = io.Copy(destination, source)
	return err
}

func snapshot(path string, output string) (fileIdentity, error) {
	parentFD, name, err := openParent(path, false)
	if err != nil {
		return fileIdentity{}, err
	}
	defer syscall.Close(parentFD)
	sourceFD, before, err := openRegularAt(parentFD, name, syscall.O_RDONLY)
	if err != nil {
		return fileIdentity{}, err
	}
	defer syscall.Close(sourceFD)

	outputParentFD, outputName, err := openParent(output, false)
	if err != nil {
		return fileIdentity{}, err
	}
	defer syscall.Close(outputParentFD)
	outputFD, err := syscall.Openat(outputParentFD, outputName, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return fileIdentity{}, fmt.Errorf("create secure snapshot: %w", err)
	}
	removeOutput := true
	defer func() {
		_ = syscall.Close(outputFD)
		if removeOutput {
			_ = syscall.Unlinkat(outputParentFD, outputName)
		}
	}()
	if err := copyFD(outputFD, sourceFD); err != nil {
		return fileIdentity{}, fmt.Errorf("copy secure snapshot: %w", err)
	}
	if err := syscall.Fchmod(outputFD, 0600); err != nil {
		return fileIdentity{}, fmt.Errorf("protect secure snapshot: %w", err)
	}
	if err := syscall.Fsync(outputFD); err != nil {
		return fileIdentity{}, fmt.Errorf("sync secure snapshot: %w", err)
	}
	var afterStat syscall.Stat_t
	if err := syscall.Fstat(sourceFD, &afterStat); err != nil {
		return fileIdentity{}, fmt.Errorf("reinspect sensitive file: %w", err)
	}
	if before != identityFromStat(&afterStat) {
		return fileIdentity{}, errors.New("sensitive file changed while it was read")
	}
	removeOutput = false
	return before, nil
}

func identityAt(parentFD int, name string) (fileIdentity, uint32, bool, error) {
	fd, err := syscall.Openat(parentFD, name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if errors.Is(err, syscall.ENOENT) {
		return fileIdentity{}, 0, false, nil
	}
	if err != nil {
		return fileIdentity{}, 0, false, errors.New(noSymlinkMessage)
	}
	defer syscall.Close(fd)
	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil {
		return fileIdentity{}, 0, false, fmt.Errorf("inspect sensitive file: %w", err)
	}
	if stat.Mode&syscall.S_IFMT != syscall.S_IFREG {
		return fileIdentity{}, 0, false, errors.New("sensitive file must be regular")
	}
	return identityFromStat(&stat), stat.Mode, true, nil
}

func atomicWrite(path string, input string, expectedValue string, mode uint32, createParents bool) error {
	if mode != 0600 {
		return errors.New("sensitive file mode must be exactly 0600")
	}
	inputParentFD, inputName, err := openParent(input, false)
	if err != nil {
		return err
	}
	defer syscall.Close(inputParentFD)
	inputFD, _, err := openRegularAt(inputParentFD, inputName, syscall.O_RDONLY)
	if err != nil {
		return err
	}
	defer syscall.Close(inputFD)

	parentFD, name, err := openParent(path, createParents)
	if err != nil {
		return err
	}
	defer syscall.Close(parentFD)
	current, currentMode, exists, err := identityAt(parentFD, name)
	if err != nil {
		return err
	}
	if expectedValue != "" {
		expected, parseErr := parseIdentity(expectedValue)
		if parseErr != nil {
			return parseErr
		}
		if !exists || current != expected {
			return errors.New("sensitive file changed after snapshot")
		}
	} else if exists {
		if currentMode&0777 != mode {
			return errors.New("existing sensitive file permissions must be exactly 0600")
		}
	}

	temporaryName := fmt.Sprintf(".%s.tmp.%d.%d", name, os.Getpid(), time.Now().UnixNano())
	temporaryFD, err := syscall.Openat(parentFD, temporaryName, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, mode)
	if err != nil {
		return fmt.Errorf("create atomic sensitive file: %w", err)
	}
	removeTemporary := true
	defer func() {
		_ = syscall.Close(temporaryFD)
		if removeTemporary {
			_ = syscall.Unlinkat(parentFD, temporaryName)
		}
	}()
	if err := copyFD(temporaryFD, inputFD); err != nil {
		return fmt.Errorf("copy atomic sensitive file: %w", err)
	}
	if err := syscall.Fchmod(temporaryFD, mode); err != nil {
		return fmt.Errorf("protect atomic sensitive file: %w", err)
	}
	if err := syscall.Fsync(temporaryFD); err != nil {
		return fmt.Errorf("sync atomic sensitive file: %w", err)
	}

	latest, _, latestExists, err := identityAt(parentFD, name)
	if err != nil {
		return err
	}
	if latestExists != exists || (exists && latest != current) {
		return errors.New("sensitive file changed during atomic write")
	}
	if err := syscall.Renameat(parentFD, temporaryName, parentFD, name); err != nil {
		return fmt.Errorf("replace sensitive file atomically: %w", err)
	}
	removeTemporary = false
	if err := syscall.Fsync(parentFD); err != nil {
		return fmt.Errorf("sync sensitive file directory: %w", err)
	}
	return nil
}

func parseMode(value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 8, 32)
	if err != nil {
		return 0, errors.New("invalid sensitive file mode")
	}
	return uint32(parsed), nil
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: secure-file snapshot|atomic-write")
	}
	switch os.Args[1] {
	case "snapshot":
		flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
		path := flags.String("path", "", "source path")
		output := flags.String("output", "", "snapshot output")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		identity, err := snapshot(*path, *output)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, identity.String())
		return err
	case "atomic-write":
		flags := flag.NewFlagSet("atomic-write", flag.ContinueOnError)
		path := flags.String("path", "", "target path")
		input := flags.String("input", "", "input path")
		expected := flags.String("expected", "", "expected target identity")
		modeValue := flags.String("mode", "0600", "target mode")
		createParents := flags.Bool("create-parents", false, "create missing parents")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		mode, err := parseMode(*modeValue)
		if err != nil {
			return err
		}
		return atomicWrite(*path, *input, *expected, mode, *createParents)
	default:
		return errors.New("usage: secure-file snapshot|atomic-write")
	}
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
