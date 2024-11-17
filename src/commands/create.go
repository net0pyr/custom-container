package commands

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func Create() {
	cmd := exec.Command("/proc/self/exe", "child")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Creating isolated parent process...")
	if err := cmd.Run(); err != nil {
		log.Println("Error creating parent process:", err)
		return
	}
}

func Child() {
	log.Println("Inside isolated process...")

	newRoot := "/tmp/container"
	if err := os.MkdirAll(newRoot+"/proc", 0700); err != nil {
		log.Println("Error creating new root:", err)
		return
	}
	if err := os.MkdirAll(newRoot+"/bin", 0700); err != nil {
		log.Println("Error creating new root:", err)
		return
	}
	defer finish(newRoot)

	log.Println("Setting up new root...")

	// Список команд для копирования
	commands := []string{"/bin/bash", "/bin/ps", "/bin/ls", "/bin/whoami"}

	// Копирование команд и их зависимостей
	for _, cmd := range commands {
		dest := newRoot + cmd
		if err := copyFile(cmd, dest); err != nil {
			log.Printf("Error copying %s: %v\n", cmd, err)
			return
		}
		if err := os.Chmod(dest, 0755); err != nil {
			log.Printf("Error setting permissions for %s: %v\n", dest, err)
			return
		}
		if err := copyDependencies(cmd, newRoot); err != nil {
			log.Printf("Error copying dependencies for %s: %v\n", cmd, err)
			return
		}
	}

	log.Println("Mounting proc...")

	if err := syscall.Mount("proc", newRoot+"/proc", "proc", 0, ""); err != nil {
		log.Println("Error mounting proc:", err)
		return
	}

	if err := syscall.Chroot(newRoot); err != nil {
		log.Println("Error changing root:", err)
		return
	}
	if err := os.Chdir("/"); err != nil {
		log.Println("Error changing directory:", err)
		return
	}

	// Создаем дочерний процесс
	cmd := exec.Command("/bin/bash") // Например, запускаем bash
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Running child process inside isolated system...")
	if err := cmd.Run(); err != nil {
		log.Println("Error running child process:", err)
		return
	}
}

func copyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func copyDependencies(binary, newRoot string) error {
	cmd := exec.Command("ldd", binary)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		for _, field := range fields {
			if strings.HasPrefix(field, "/") {
				dest := filepath.Join(newRoot, field)
				if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
					return err
				}
				if err := copyFile(field, dest); err != nil {
					return err
				}
				if err := os.Chmod(dest, 0755); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func finish(dir string) {
	log.Println("Finishing...")

	if err := syscall.Unmount(dir+"/proc", 0); err != nil {
		log.Println("Error unmounting proc:", err)
		return
	}

	if err := os.RemoveAll(dir); err != nil {
		log.Println("Error removing new root directory:", err)
		return
	}
}
