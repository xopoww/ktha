package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/xopoww/ktha/node/internal/common"
	"go.uber.org/zap"
)

var args struct {
	image       string
	containerID string
	socket      string
	inner       bool

	limits common.ContainerLimits

	nodeBinary string
	rootfsRoot string
	cgroupBase string
}

const defaultMemoryMax = 100 * 1024 * 1024 // 100 MB
const defaultPidsMax = 30
const defaultCPUMax = 20000 // 20% of one core

func init() {
	flag.StringVar(&args.image, "image", "", "path to unpacked image (don't pass with --inner)")
	flag.StringVar(&args.containerID, "id", "", "container id")
	flag.StringVar(&args.socket, "sock", "app.sock", "path (relative to root) to unix socket for the guest")
	flag.BoolVar(&args.inner, "inner", false, "if command is invoked inside a namespace already (never pass manually)")

	flag.IntVar(&args.limits.MemoryMax, "mem-max", defaultMemoryMax, "max memory (in bytes)")
	flag.IntVar(&args.limits.PidsMax, "pids-max", defaultPidsMax, "max number of processes")
	flag.IntVar(&args.limits.CPUMax, "cpu-max", defaultCPUMax, "max cpu time (in µs in 100000µs window)")

	flag.StringVar(&args.nodeBinary, "node-bin", "", "path to node.js runtime binary (resolves from path by default)")
	flag.StringVar(&args.rootfsRoot, "rootfs", "/tmp/ktha/rootfs/", "parent directory for container roots")
	flag.StringVar(&args.cgroupBase, "cgroup", "/sys/fs/cgroup/ktha", "cgroup base path")

	flag.Parse()
}

func runChild(log *zap.Logger, child *exec.Cmd) error {
	if child.SysProcAttr == nil {
		child.SysProcAttr = &syscall.SysProcAttr{}
	}
	child.SysProcAttr.Pdeathsig = syscall.SIGKILL

	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	// forward signals to child for graceful shutdown
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigch
		log.Sugar().Debugf("Received signal %s", sig)
		if child.Process != nil {
			child.Process.Signal(sig)
		}
	}()

	// lock thread for Pdeathsig to work
	runtime.LockOSThread()

	log.Sugar().Debugf("Running '%s'...", child)
	if err := child.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	if err := child.Wait(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				log.Sugar().Debugf("Child killed by signal %s", status.Signal())
				return nil
			}
		}
		return fmt.Errorf("wait: %w", err)
	}

	return nil
}

func ensureMountPoint(target string, isDir bool) error {
	if isDir {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

func bindMountReadonly(source string, target string) error {
	if err := syscall.Mount(source, target, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	if err := syscall.Mount("", target, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("remount ro: %w", err)
	}
	return nil
}

func rootfsLocation(containerID string) string {
	return filepath.Join(args.rootfsRoot, containerID)
}

func setupRootfs(log *zap.Logger, image string, root string) error {
	log.Sugar().Debugf("Setting up rootfs at %q from %q...", root, image)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.CopyFS(root, os.DirFS(image)); err != nil {
		return fmt.Errorf("copy image: %w", err)
	}
	return nil
}

func cleanupRootfs(log *zap.Logger, root string) {
	log.Sugar().Debugf("Cleaning up rootfs at %q...", root)
	if err := os.RemoveAll(root); err != nil {
		log.Sugar().Errorf("Error during rootfs cleanup: %s.", err)
	}
}

func setupCgroup(log *zap.Logger, baseDir string, containerID string, limits common.ContainerLimits) (cgroupFD int, cleanup func(), err error) {
	cgroupPath := filepath.Join(baseDir, containerID)
	if err := os.MkdirAll(cgroupPath, 0o755); err != nil {
		return 0, nil, fmt.Errorf("mkdir: %w", err)
	}

	cleanup = func() {
		if err := os.Remove(cgroupPath); err != nil {
			log.Sugar().Errorf("Failed to remove cgroup: %s.", err)
		}
	}

	writeCgroup := func(controller string, value string) error {
		controllerPath := filepath.Join(cgroupPath, controller)
		log.Sugar().Debugf("Write %q -> %s.", value, controllerPath)
		if err := os.WriteFile(controllerPath, []byte(value), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", controller, err)
		}
		return nil
	}

	if err := writeCgroup("memory.max", fmt.Sprint(limits.MemoryMax)); err != nil {
		return 0, cleanup, err
	}
	if err := writeCgroup("memory.oom.group", "1"); err != nil {
		return 0, cleanup, err
	}
	if err := writeCgroup("pids.max", fmt.Sprint(limits.PidsMax)); err != nil {
		return 0, cleanup, err
	}
	if err := writeCgroup("cpu.max", fmt.Sprintf("%d 100000", limits.CPUMax)); err != nil {
		return 0, cleanup, err
	}

	cgroupFD, err = syscall.Open(cgroupPath, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return 0, cleanup, fmt.Errorf("open cgroup: %w", err)
	}

	oldCleanup := cleanup
	cleanup = func() {
		if err := syscall.Close(cgroupFD); err != nil {
			log.Sugar().Errorf("Failed to close cgroup fd: %s.", err)
		}
		oldCleanup()
	}

	return cgroupFD, cleanup, nil
}

func outer(log *zap.Logger) error {
	if args.image == "" {
		return fmt.Errorf("image is required")
	}

	// set up rootfs

	root := rootfsLocation(args.containerID)

	if err := setupRootfs(log, args.image, root); err != nil {
		cleanupRootfs(log, root)
		return fmt.Errorf("set up rootfs: %w", err)
	}
	defer cleanupRootfs(log, root)

	// set up cgroup with limits

	cgroupFD, cleanupCgroup, err := setupCgroup(log, args.cgroupBase, args.containerID, args.limits)
	if err != nil {
		cleanupCgroup()
		return fmt.Errorf("setup cgroup: %w", err)
	}
	defer cleanupCgroup()

	// call itself in "inner" mode

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("os.Executable: %w", err)
	}

	argv := make([]string, 0)
	argv = append(argv, "--id", args.containerID)
	argv = append(argv, "--sock", args.socket)
	argv = append(argv, "--inner")
	argv = append(argv, "--node-bin", args.nodeBinary)
	child := exec.Command(self, argv...)

	child.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWNS | syscall.CLONE_NEWPID,
		CgroupFD:    cgroupFD,
		UseCgroupFD: true,
	}

	return runChild(log, child)
}

func inner(log *zap.Logger) error {
	if os.Getpid() != 1 {
		return fmt.Errorf("must be in a new pid namespace")
	}

	// prevent mount propagation back to host

	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("set mount propagation to private: %w", err)
	}

	// bind mount node and shared libraries

	const CanonicalNodeBinary = "/usr/bin/node"
	var SharedLibDirs = []string{"/lib", "/lib64"}

	root := rootfsLocation(args.containerID)

	nodeTarget := filepath.Join(root, CanonicalNodeBinary)
	if err := ensureMountPoint(nodeTarget, false); err != nil {
		return fmt.Errorf("ensure node mount point: %w", err)
	}
	if err := bindMountReadonly(args.nodeBinary, nodeTarget); err != nil {
		return fmt.Errorf("mount node: %w", err)
	}

	for _, libDir := range SharedLibDirs {
		libTarget := filepath.Join(root, libDir)
		if err := ensureMountPoint(libTarget, true); err != nil {
			return fmt.Errorf("ensure %q mount point: %w", libDir, err)
		}
		if err := bindMountReadonly(libDir, libTarget); err != nil {
			return fmt.Errorf("mount %q: %w", libDir, err)
		}
	}

	// mount proc

	procTarget := filepath.Join(root, "proc")
	if err := ensureMountPoint(procTarget, true); err != nil {
		return fmt.Errorf("ensure proc mount point: %w", err)
	}
	if err := syscall.Mount("proc", procTarget, "proc", 0, ""); err != nil {
		return fmt.Errorf("mount proc: %w", err)
	}

	// chroot

	if err := syscall.Chroot(root); err != nil {
		return fmt.Errorf("chroot: %w", err)
	}
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir: %w", err)
	}

	// run node

	const SocketEnvVar = "KTHA_SOCK"

	child := exec.Command(CanonicalNodeBinary, "/index.js")
	child.Env = []string{fmt.Sprintf("%s=%s", SocketEnvVar, filepath.Join("/", args.socket))}

	return runChild(log, child)
}

func run() error {
	log, err := zap.NewDevelopment()
	if err != nil {
		return fmt.Errorf("init zap: %w", err)
	}
	log = log.With(zap.Bool("inner", args.inner))
	defer log.Sync()

	if args.containerID == "" {
		return fmt.Errorf("id is required")
	}
	log = log.With(zap.String("containerID", args.containerID))

	if args.nodeBinary == "" {
		var err error
		args.nodeBinary, err = exec.LookPath("node")
		if err != nil {
			return fmt.Errorf("lookup node: %w; pass the --node-bin manually", err)
		}
	}

	if args.inner {
		return inner(log)
	} else {
		return outer(log)
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Fatal error: %s.", err)
	}
}
