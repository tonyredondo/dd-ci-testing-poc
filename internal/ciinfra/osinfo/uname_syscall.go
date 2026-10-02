//go:build linux || aix

package osinfo

import "syscall"

func getKernelInfo() (kernelInfo, error) {
	var uts syscall.Utsname
	if err := syscall.Uname(&uts); err != nil {
		return kernelInfo{}, err
	}
	return kernelInfo{utsString(uts.Sysname[:]), utsString(uts.Release[:]), utsString(uts.Version[:])}, nil
}
