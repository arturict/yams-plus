package preflight

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type PortBinding struct {
	Name     string
	Port     int
	Protocol string
}

func Run(ctx context.Context, production bool, bindAddresses []string, portBindings []PortBinding) []Check {
	checks := []Check{{Name: "architecture", Status: "healthy", Message: runtime.GOOS + "/" + runtime.GOARCH}}
	if production && (runtime.GOOS != "linux" || runtime.GOARCH != "amd64") {
		checks[0].Status = "failed"
		checks[0].Message += " is unsupported for production"
	}
	checks = append(checks, commandCheck(ctx, "docker", "docker", "version", "--format", "{{.Server.Version}}"))
	checks = append(checks, commandCheck(ctx, "compose", "docker", "compose", "version", "--short"))
	cpu := Check{Name: "cpu", Status: "healthy", Message: fmt.Sprintf("%d logical CPU(s)", runtime.NumCPU())}
	if runtime.NumCPU() < 4 {
		cpu.Status, cpu.Message = "action-required", cpu.Message+"; 4 are recommended"
	}
	checks = append(checks, cpu)
	checks = append(checks, dnsCheck(ctx))
	if production {
		raw, err := os.ReadFile("/etc/os-release")
		osCheck := Check{Name: "operating-system", Status: "failed", Message: "cannot read /etc/os-release"}
		if err == nil {
			text := string(raw)
			if (strings.Contains(text, "ID=debian") && strings.Contains(text, "VERSION_ID=\"13\"")) || (strings.Contains(text, "ID=ubuntu") && strings.Contains(text, "VERSION_ID=\"24.04\"")) {
				osCheck.Status = "healthy"
				osCheck.Message = "supported distribution"
			} else {
				osCheck.Message = "requires Debian 13 or Ubuntu 24.04"
			}
		}
		checks = append(checks, osCheck)
		checks = append(checks, linuxMemoryCheck(), linuxStorageCheck(ctx), linuxFilesystemCheck(ctx), linuxTimeCheck(ctx), linuxGPUCheck())
		if _, err := exec.LookPath("tailscale"); err == nil {
			checks = append(checks, tailscaleCheck(ctx))
		} else {
			checks = append(checks, Check{Name: "tailscale", Status: "healthy", Message: "not installed; LAN-only binding remains available"})
		}
	}
	for _, address := range bindAddresses {
		status := "healthy"
		message := "private bind address"
		ip := net.ParseIP(address)
		if ip == nil || (!ip.IsLoopback() && !ip.IsPrivate() && !strings.HasPrefix(address, "100.")) {
			status, message = "failed", "public or invalid address"
		}
		checks = append(checks, Check{Name: "bind-" + address, Status: status, Message: message})
	}
	checks = append(checks, portChecks(bindAddresses, portBindings)...)
	return checks
}

func portChecks(bindAddresses []string, bindings []PortBinding) []Check {
	checks := make([]Check, 0, len(bindAddresses)*len(bindings))
	for _, address := range bindAddresses {
		for _, binding := range bindings {
			protocol := binding.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			name := fmt.Sprintf("port-%s-%s-%d", binding.Name, protocol, binding.Port)
			target := net.JoinHostPort(address, strconv.Itoa(binding.Port))
			check := Check{Name: name, Status: "healthy", Message: target + " is available"}
			if protocol == "udp" {
				listener, err := net.ListenPacket("udp", target)
				if err != nil {
					check.Status, check.Message = "failed", target+" is unavailable: "+err.Error()
				} else {
					_ = listener.Close()
				}
			} else {
				listener, err := net.Listen("tcp", target)
				if err != nil {
					check.Status, check.Message = "failed", target+" is unavailable: "+err.Error()
				} else {
					_ = listener.Close()
				}
			}
			checks = append(checks, check)
		}
	}
	return checks
}

func tailscaleCheck(ctx context.Context) Check {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "tailscale", "status", "--json").Run(); err != nil {
		return Check{Name: "tailscale", Status: "action-required", Message: "installed but not connected"}
	}
	return Check{Name: "tailscale", Status: "healthy", Message: "connected; peer details redacted"}
}

func dnsCheck(ctx context.Context) Check {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupHost(ctx, "registry-1.docker.io")
	if err != nil || len(addresses) == 0 {
		return Check{Name: "dns", Status: "failed", Message: fmt.Sprint(err)}
	}
	return Check{Name: "dns", Status: "healthy", Message: "Docker registry resolves"}
}

func linuxMemoryCheck() Check {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return Check{Name: "memory", Status: "failed", Message: err.Error()}
	}
	fields := strings.Fields(string(raw))
	var kib int64
	for index := 0; index+1 < len(fields); index++ {
		if fields[index] == "MemTotal:" {
			kib, _ = strconv.ParseInt(fields[index+1], 10, 64)
			break
		}
	}
	gib := float64(kib) / 1024 / 1024
	check := Check{Name: "memory", Status: "healthy", Message: fmt.Sprintf("%.1f GiB RAM", gib)}
	if gib < 8 {
		check.Status, check.Message = "action-required", check.Message+"; 8 GiB are recommended"
	}
	return check
}

func linuxStorageCheck(ctx context.Context) Check {
	check := commandCheck(ctx, "storage", "df", "-Pk", "/")
	if check.Status == "healthy" {
		lines := strings.Split(check.Message, "\n")
		if len(lines) > 1 {
			fields := strings.Fields(lines[len(lines)-1])
			if len(fields) >= 4 {
				available, _ := strconv.ParseInt(fields[3], 10, 64)
				check.Message = fmt.Sprintf("%.1f GiB free on root filesystem", float64(available)/1024/1024)
				if available < 20*1024*1024 {
					check.Status, check.Message = "action-required", check.Message+"; at least 20 GiB free is recommended before media"
				}
			}
		}
	}
	return check
}

func linuxFilesystemCheck(ctx context.Context) Check {
	check := commandCheck(ctx, "filesystem", "findmnt", "-n", "-o", "FSTYPE", "/")
	if check.Status == "healthy" {
		fs := strings.TrimSpace(check.Message)
		check.Message = fs
		if fs == "vfat" || strings.HasPrefix(fs, "fuse") {
			check.Status, check.Message = "failed", fs+" is unsuitable for application databases"
		}
	}
	return check
}

func linuxTimeCheck(ctx context.Context) Check {
	check := commandCheck(ctx, "time-sync", "timedatectl", "show", "--property=NTPSynchronized", "--value")
	if check.Status == "healthy" && strings.EqualFold(strings.TrimSpace(check.Message), "no") {
		check.Status, check.Message = "failed", "NTP is not synchronized"
	}
	return check
}

func linuxGPUCheck() Check {
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		return Check{Name: "gpu", Status: "healthy", Message: "DRM render device detected"}
	}
	return Check{Name: "gpu", Status: "healthy", Message: "no DRM render device detected; software transcoding remains available"}
}

func commandCheck(ctx context.Context, name, command string, args ...string) Check {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if err != nil {
		return Check{Name: name, Status: "failed", Message: err.Error()}
	}
	return Check{Name: name, Status: "healthy", Message: strings.TrimSpace(string(out))}
}

func Failed(checks []Check) error {
	var names []string
	for _, check := range checks {
		if check.Status == "failed" {
			names = append(names, check.Name+": "+check.Message)
		}
	}
	if len(names) > 0 {
		return fmt.Errorf("preflight failed: %s", strings.Join(names, "; "))
	}
	return nil
}
