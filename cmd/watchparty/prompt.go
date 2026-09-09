//go:build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// isTerminal informa se o stdin é um terminal (para decidir perguntar senha).
func isTerminal(fd uintptr) bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&t)), 0, 0, 0)
	return errno == 0
}

// readHidden lê uma linha do stdin sem eco (label vai para o stderr).
func readHidden(label string) (string, error) {
	fd := os.Stdin.Fd()
	var old syscall.Termios
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&old)), 0, 0, 0); errno != 0 {
		return "", errno
	}
	noecho := old
	noecho.Lflag &^= syscall.ECHO
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(&noecho)), 0, 0, 0); errno != 0 {
		return "", errno
	}
	defer func() {
		_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(&old)), 0, 0, 0)
	}()

	fmt.Fprint(os.Stderr, label)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// choosePassword pergunta a senha (sem eco) e pede confirmação.
// Enter duas vezes = senha vazia (sala sem senha).
func choosePassword() (string, error) {
	for {
		p1, err := readHidden("Senha da sala (Enter = sem senha): ")
		if err != nil {
			return "", err
		}
		p2, err := readHidden("Confirme a senha: ")
		if err != nil {
			return "", err
		}
		if p1 == p2 {
			return p1, nil
		}
		fmt.Fprintln(os.Stderr, "as senhas não conferem; tente novamente")
	}
}
