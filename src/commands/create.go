package commands

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/net0pyr/custom-container/commands/creatingModule"
	"golang.org/x/sys/unix"
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
	// fileDNS, err := os.OpenFile(newRoot+"/etc/resolv.conf", os.O_CREATE|os.O_WRONLY, 0644)
	// if err != nil {
	// 	log.Println("Error creating resolv.conf file:", err)
	// 	return
	// }
	// if _, err = fileDNS.WriteString("nameserver 192.168.1.1\n"); err != nil {
	// 	log.Println("Error writing to resolv.conf file:", err)
	// 	return
	// }
	defer filePasswd.Close()
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

	pid, err := getPID("child")
	if err != nil {
		log.Println("Error getting PID:", err)
		return
	}

	log.Println("Child PID:", pid)

	HOST_IP := "192.168.1.123"
	CONTAINER_PID := pid
	CONTAINER_IP := "192.168.1.124"
	SUBNET := "192.168.1.0/24"

	// Создание пары интерфейсов veth
	if err := runCommand("ip", "link", "add", "veth-host", "type", "veth", "peer", "name", "veth-container"); err != nil {
		fmt.Println("Error creating veth pair:", err)
		return
	}

	// Поднятие интерфейса на хосте
	if err := runCommand("ip", "link", "set", "veth-host", "up"); err != nil {
		fmt.Println("Error setting veth-host up:", err)
		return
	}
	if err := runCommand("ip", "addr", "add", fmt.Sprintf("%s/24", HOST_IP), "dev", "veth-host"); err != nil {
		fmt.Println("Error adding IP address to veth-host:", err)
		return
	}

	// Перемещение интерфейса контейнера в сетевой namespace контейнера
	if err := runCommand("ip", "link", "set", "veth-container", "netns", CONTAINER_PID); err != nil {
		fmt.Println("Error moving veth-container to container namespace:", err)
		return
	}

	// Настройка интерфейса в контейнере
	if err := runCommand("nsenter", "--net=/proc/"+CONTAINER_PID+"/ns/net", "ip", "link", "set", "veth-container", "up"); err != nil {
		fmt.Println("Error setting veth-container up in container:", err)
		return
	}
	if err := runCommand("nsenter", "--net=/proc/"+CONTAINER_PID+"/ns/net", "ip", "addr", "add", fmt.Sprintf("%s/24", CONTAINER_IP), "dev", "veth-container"); err != nil {
		fmt.Println("Error adding IP address to veth-container in container:", err)
		return
	}
	if err := runCommand("nsenter", "--net=/proc/"+CONTAINER_PID+"/ns/net", "ip", "route", "add", "default", "via", HOST_IP); err != nil {
		fmt.Println("Error adding default route in container:", err)
		return
	}

	// Проверка и настройка маршрутизации и NAT на хосте
	output, err := commandOutput("iptables", "-t", "nat", "-L", "POSTROUTING")
	if err != nil {
		fmt.Println("Error checking iptables rules:", err)
		return
	}
	if !strings.Contains(output, fmt.Sprintf("MASQUERADE  all  --  %s", SUBNET)) {
		if err := runCommand("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", SUBNET, "-j", "MASQUERADE"); err != nil {
			fmt.Println("Error setting up NAT:", err)
			return
		}
	}

	output, err = commandOutput("sysctl", "net.ipv4.ip_forward")
	if err != nil {
		fmt.Println("Error checking IP forwarding:", err)
		return
	}
	if !strings.Contains(output, "net.ipv4.ip_forward = 1") {
		if err := runCommand("sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
			fmt.Println("Error enabling IP forwarding:", err)
			return
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Println("Error waiting for parent process:", err)
		return
	}
}

func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func commandOutput(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	return string(output), err
}

func getPID(processName string) (string, error) {
	// Выполнение команды ps -aux | grep processName
	cmd := exec.Command("sh", "-c", fmt.Sprintf("ps -aux | grep %s", processName))
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Парсинг вывода команды
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, processName) && !strings.Contains(line, "grep") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				return fields[1], nil
			}
		}
	}

	return "", fmt.Errorf("process not found")
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
