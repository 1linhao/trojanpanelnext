package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"

	"trojanpanelnext/revocationreceipt"
)

func runVerifyReceipt(args []string) error {
	set := flag.NewFlagSet("node-bundle verify-receipt", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	var receiptPath, publicPath, identityID, serverText, generationText string
	set.StringVar(&receiptPath, "receipt-file", "", "Web-issued revocation receipt")
	set.StringVar(&publicPath, "pinned-public-key", "", "installed Web revocation public key")
	set.StringVar(&identityID, "identity-id", "", "installed Node identity ID")
	set.StringVar(&serverText, "server-id", "", "installed Node server ID")
	set.StringVar(&generationText, "generation", "", "installed Node generation")
	if set.Parse(args) != nil || len(set.Args()) != 0 || receiptPath == "" || publicPath == "" || identityID == "" {
		return errors.New("verify-receipt requires receipt, pinned key, identity, server ID, and generation")
	}
	serverID, serverErr := strconv.ParseUint(serverText, 10, 64)
	generation, generationErr := strconv.ParseUint(generationText, 10, 64)
	if serverErr != nil || generationErr != nil || serverID == 0 || generation == 0 {
		return errors.New("installed Node identity metadata is invalid")
	}
	publicData, err := readProofFile(publicPath, 256)
	if err != nil {
		return errors.New("installed revocation public key is missing or unsafe")
	}
	public, err := revocationreceipt.ParsePublicKey(publicData)
	if err != nil {
		return errors.New("installed revocation public key is invalid")
	}
	receipt, err := readProofFile(receiptPath, revocationreceipt.MaxReceiptSize)
	if err != nil {
		return errors.New("revocation receipt is missing or unsafe")
	}
	if _, err = revocationreceipt.Verify(receipt, public, revocationreceipt.Node{
		IdentityID: identityID, ServerID: serverID, Generation: generation,
	}); err != nil {
		return errors.New("revocation receipt does not authorize this installed Node identity")
	}
	fmt.Fprintln(os.Stdout, "Node revocation receipt verified for installed identity")
	return nil
}

func readProofFile(path string, limit int) ([]byte, error) {
	if path == "" || path[0] != '/' {
		return nil, errors.New("proof path must be absolute")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() <= 0 || info.Size() > int64(limit) {
		return nil, errors.New("proof file is unsafe")
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("proof file ownership is unsafe")
	}
	return io.ReadAll(io.LimitReader(file, int64(limit)+1))
}
