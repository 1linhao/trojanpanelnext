//go:build linux && (amd64 || arm64)

package xray

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"

	"trojan-panel-core/core"
)

// Start the child from a disposable OS thread. Fork/exec inherits its Landlock
// and seccomp rules. The long-running Agent's threads retain their normal access.
func startRestrictedCheck(cmd *exec.Cmd) error {
	started := make(chan error, 1)
	go func() {
		runtime.LockOSThread() // Exiting the goroutine discards the restricted thread.
		if err := restrictCheckProcess(cmd.Path); err != nil {
			started <- fmt.Errorf("read-only Xray validation sandbox unavailable: %w", err)
			return
		}
		started <- cmd.Start()
	}()
	return <-started
}

func restrictCheckProcess(binary string) error {
	// ABI 3 also covers file truncation. Fail closed on older/disabled kernels.
	rules := unix.LandlockRulesetAttr{Access_fs: (unix.LANDLOCK_ACCESS_FS_TRUNCATE << 1) - 1}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&rules)), unsafe.Sizeof(rules), 0)
	if errno != 0 {
		return errno
	}
	defer unix.Close(int(fd))
	allow := func(path string, access uint64) error {
		if path == "" {
			return nil
		}
		file, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		if err != nil {
			return err
		}
		defer unix.Close(file)
		var stat unix.Stat_t
		if err = unix.Fstat(file, &stat); err != nil {
			return err
		}
		if stat.Mode&unix.S_IFMT != unix.S_IFREG {
			return errors.New("validation resources must be regular files")
		}
		rule := unix.LandlockPathBeneathAttr{Parent_fd: int32(file), Allowed_access: access}
		_, _, errNo := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, fd, unix.LANDLOCK_RULE_PATH_BENEATH, uintptr(unsafe.Pointer(&rule)), 0, 0, 0)
		if errNo != 0 {
			return errNo
		}
		return nil
	}
	for _, path := range []string{binary, "/lib64/ld-linux-x86-64.so.2", "/lib/ld-linux-aarch64.so.1", "/lib/ld-musl-x86_64.so.1", "/lib/ld-musl-aarch64.so.1", "/usr/lib/libc.so.6", "/lib/x86_64-linux-gnu/libc.so.6", "/lib/aarch64-linux-gnu/libc.so.6"} {
		if err := allow(path, unix.LANDLOCK_ACCESS_FS_READ_FILE|unix.LANDLOCK_ACCESS_FS_EXECUTE); err != nil {
			return err
		}
	}
	for _, path := range []string{core.Config.CertConfig.CrtPath, core.Config.CertConfig.KeyPath, filepath.Join(filepath.Dir(binary), "geoip.dat"), filepath.Join(filepath.Dir(binary), "geosite.dat")} {
		if err := allow(path, unix.LANDLOCK_ACCESS_FS_READ_FILE); err != nil {
			return err
		}
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	if _, _, errno = unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, fd, 0, 0); errno != 0 {
		return errno
	}
	// The installed native kernel needs no sockets to compile an offline
	// configuration. It inherits only stdio pipes from exec.Cmd.
	filter := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: unix.SYS_SOCKET, Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: 0x00050000 | uint32(unix.EACCES)},
		{Code: unix.BPF_RET | unix.BPF_K, K: 0x7fff0000},
	}
	prog := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	err := unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, uintptr(unsafe.Pointer(&prog)), 0, 0)
	runtime.KeepAlive(filter)
	return err
}
