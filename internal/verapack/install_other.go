//go:build !windows

package verapack

import "errors"

func InstallCli() error {
	return errors.New("this cli currently only supports windows")
}
