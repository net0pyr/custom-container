package commands

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

func Create() {
	log.Println("Creating a new container")

	rootDir := "/tmp/container"
	procDir := rootDir + "/proc"
	if err := os.MkdirAll(procDir, 0755); err != nil {
		log.Fatalf("Error creating temp directory: %s\n", err)
	}

	cmd := exec.Command("/bin/bash")

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWCGROUP,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Fatalf("Error creating new namespace the command: %s\n", err)
	}

	// Монтирование новой файловой системы proc в временную директорию
	if err := syscall.Mount("proc", procDir, "proc", 0, ""); err != nil {
		log.Fatalf("Error mounting proc filesystem: %s\n", err)
	}

	// Использование chroot для изменения корневой файловой системы
	if err := syscall.Chroot(rootDir); err != nil {
		log.Fatalf("Error changing root directory: %s\n", err)
	}
	if err := os.Chdir("/"); err != nil {
		log.Fatalf("Error changing directory: %s\n", err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatalf("Error waiting for command: %s\n", err)
	}

	// Размонтирование файловой системы proc
	if err := syscall.Unmount(procDir, 0); err != nil {
		log.Fatalf("Error unmounting proc filesystem: %s\n", err)
	}

	// Удаление временной директории
	if err := os.RemoveAll(rootDir); err != nil {
		log.Fatalf("Error removing temp directory: %s\n", err)
	}

	log.Printf("Shell PID: %d", cmd.Process.Pid)
}
