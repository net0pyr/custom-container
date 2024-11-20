package commands

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/net0pyr/custom-container/commands/creatingModule"
	"golang.org/x/sys/unix"
)

func Create() {
	cmd := exec.Command("/proc/self/exe", "child")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET | syscall.CLONE_NEWCGROUP,
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

	newRoot := "/tmp/container"
	if err := os.MkdirAll(newRoot+"/proc", 0755); err != nil {
		log.Println("Error creating new root:", err)
		return
	}
	if err := os.MkdirAll(newRoot+"/bin", 0755); err != nil {
		log.Println("Error creating new root:", err)
		return
	}
	if err := os.MkdirAll(newRoot+"/usr/bin", 0755); err != nil {
		log.Println("Error creating new root:", err)
		return
	}
	if err := os.MkdirAll(newRoot+"/root", 0755); err != nil {
		log.Println("Error creating new root:", err)
		return
	}
	if err := os.MkdirAll(newRoot+"/dev", 0755); err != nil {
		log.Println("Error creating new root:", err)
		return
	}
	if err := os.MkdirAll(newRoot+"/etc", 0755); err != nil {
		log.Println("Error creating new root:", err)
		return
	}
	filePasswd, err := os.OpenFile(newRoot+"/etc/passwd", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Error creating passwd file:", err)
		return
	}
	if _, err = filePasswd.WriteString("root:x:0:0:root:/root:/bin/bash\n"); err != nil {
		log.Println("Error writing to passwd file:", err)
		return
	}
	fileDNS, err := os.OpenFile(newRoot+"/etc/resolv.conf", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Error creating resolv.conf file:", err)
		return
	}
	if _, err = fileDNS.WriteString("nameserver 192.168.1.1\n"); err != nil {
		log.Println("Error writing to resolv.conf file:", err)
		return
	}
	defer filePasswd.Close()
	defer fileDNS.Close()
	defer finish(newRoot)

	if err := os.Remove(newRoot + "/dev/null"); err != nil && !os.IsNotExist(err) {
		log.Println("Error remove device file:", err)
	}

	if err := syscall.Mknod(newRoot+"/dev/null", syscall.S_IFCHR|0666, int(unix.Mkdev(1, 3))); err != nil {
		log.Println("Error create device file:", err)
	}

	if err := os.Chmod(newRoot+"/dev/null", 0666); err != nil {
		log.Println("Error setting permissions for /dev/null:", err)
		return
	}

	log.Println("Creating isolated parent process...")
	if err := cmd.Start(); err != nil {
		log.Println("Error creating parent process:", err)
		return
	}

	pid := fmt.Sprintf("%d", cmd.Process.Pid)

	log.Println("Child PID:", pid)

	cmdNetworkScript := exec.Command("/bin/bash", "networkScript.sh", pid)
	cmdNetworkScript.Stdout = nil
	cmdNetworkScript.Stderr = nil
	if err := cmdNetworkScript.Run(); err != nil {
		log.Println("Error running network script:", err)
		return
	}

	cgroupPath := "/sys/fs/cgroup/custom-container"
	cgroupController := cgroupPath + "/cgroup.subtree_control"
	cgroupPids := cgroupPath + "/cgroup.procs"
	cgroupCPU := cgroupPath + "/cpu.max"
	cgroupMemory := cgroupPath + "/memory.max"

	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		log.Fatalln("Error creating cgroup pids:", err)
	}

	if err := os.WriteFile(cgroupController, []byte("+memory +cpu"), 0644); err != nil {
		log.Fatalln("Error writing to cgroup controller:", err)
	}

	if err := os.WriteFile(cgroupCPU, []byte("25000 100000"), 0644); err != nil {
		log.Fatalln("Error writing to cgroup CPU:", err)
	}

	if err := os.WriteFile(cgroupMemory, []byte("2000000000"), 0644); err != nil {
		log.Fatalln("Error writing to cgroup memory:", err)
	}

	if err := os.WriteFile(cgroupPids, []byte(pid), 0644); err != nil {
		log.Fatalln("Error writing to cgroup pids:", err)
	}

	if err := cmd.Wait(); err != nil {
		log.Println("Error waiting for parent process:", err)
		return
	}
}

func Child() {
	log.Println("Inside isolated process...")

	newRoot := "/tmp/container"

	log.Println("Setting up new root...")

	commands := []string{"/bin/bash", "/bin/ps", "/bin/ls", "/bin/whoami", "/usr/bin/ping", "/usr/bin/cat", "/usr/bin/ip", "/usr/bin/nsenter"}

	for _, cmd := range commands {
		dest := newRoot + cmd
		if err := creatingModule.CopyFile(cmd, dest); err != nil {
			log.Printf("Error copying %s: %v\n", cmd, err)
			return
		}
		if err := os.Chmod(dest, 0755); err != nil {
			log.Printf("Error setting permissions for %s: %v\n", dest, err)
			return
		}
		if err := creatingModule.CopyDependencies(cmd, newRoot); err != nil {
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

	if err := os.Setenv("PATH", "/bin:/usr/bin"); err != nil {
		log.Println("Error setting PATH:", err)
		return
	}

	cmd := exec.Command("/bin/bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Running child process inside isolated system...")
	if err := cmd.Run(); err != nil {
		log.Println("Error running child process:", err)
		return
	}
}

func finish(dir string) {
	log.Println("Finishing...")

	if err := os.RemoveAll(dir); err != nil {
		log.Println("Error removing new root directory:", err)
		return
	}
}
