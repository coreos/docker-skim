// Copyright 2017 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/appc/spec/schema/types"

	"github.com/coreos/rkt/common"
	pkgflag "github.com/coreos/rkt/pkg/flag"
	rktlog "github.com/coreos/rkt/pkg/log"
	"github.com/coreos/rkt/pkg/sys"
	"github.com/coreos/rkt/pkg/user"
	stage1common "github.com/coreos/rkt/stage1/common"
	stage1commontypes "github.com/coreos/rkt/stage1/common/types"
)

const (
	flavor = "skim"
)

var (
	debug bool

	discardNetlist common.NetList
	discardBool    bool
	discardString  string

	log  *rktlog.Logger
	diag *rktlog.Logger
)

func parseFlags() *stage1commontypes.RuntimePod {
	rp := stage1commontypes.RuntimePod{}

	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
	flag.BoolVar(&discardBool, "interactive", true, "The pod is interactive (single image only)")

	// The following flags need to be supported by stage1 according to
	// https://github.com/coreos/rkt/blob/master/Documentation/devel/stage1-implementors-guide.md
	// Most of them are ignored
	// These are ignored, but stage0 always passes them
	flag.Var(&discardNetlist, "net", "Setup networking")
	flag.StringVar(&discardString, "local-config", common.DefaultLocalConfigDir, "Local config path")

	// These are discarded with a warning
	// TODO either implement these, or stop passing them

	flag.Var(pkgflag.NewDiscardFlag("mds-token"), "mds-token", "MDS auth token (not implemented)")
	flag.Var(pkgflag.NewDiscardFlag("hostname"), "hostname", "Set hostname (not implemented)")
	flag.Bool("disable-capabilities-restriction", true, "ignored")
	flag.Bool("disable-paths", true, "ignored")
	flag.Bool("disable-seccomp", true, "ignored")

	// Since we're running on the host natively, we wll also ingnore tweaking dns/host
	dnsConfMode := pkgflag.MustNewPairList(map[string][]string{
		"resolv": {"host", "stage0", "none", "default"},
		"hosts":  {"host", "stage0", "default"},
	}, map[string]string{
		"resolv": "default",
		"hosts":  "default",
	})
	flag.Var(dnsConfMode, "dns-conf-mode", "DNS config file modes")

	flag.Parse()

	rp.Debug = debug

	return &rp
}

func stage1(rp *stage1commontypes.RuntimePod) int {
	rootDir, _ := os.Getwd()

	uuid, err := types.NewUUID(flag.Arg(0))
	if err != nil {
		log.Print("UUID is missing or malformed\n")
		return 254
	}

	root := "."
	p, err := stage1commontypes.LoadPod(root, uuid, rp)
	if err != nil {
		log.PrintE("can't load pod", err)
		return 254
	}

	if err := p.SaveRuntime(); err != nil {
		log.FatalE("failed to save runtime parameters", err)
		return 254
	}

	// If we're running in interactive mode, we can support only 1 app per pod
	if len(p.Manifest.Apps) != 1 {
		log.FatalE("too many apps to support in stage1-skim", nil)
		return 254
	}

	// lock the current goroutine to its current OS thread.
	// This will force the subsequent syscalls to be executed in the same OS thread as Setresuid, and Setresgid,
	// see https://github.com/golang/go/issues/1435#issuecomment-66054163.
	runtime.LockOSThread()

	lfd, err := common.GetRktLockFD()
	if err != nil {
		log.PrintE("can't get rkt lock fd", err)
		return 254
	}

	ra := p.Manifest.Apps[0]
	args := ra.App.Exec
	if len(args) == 0 {
		log.Printf(`image %q has an empty "exec" (try --exec=BINARY)`, p.AppNameToImageName(ra.Name))
		return 254
	}

	// change permissions for the root directory to be world readable/executable
	// This is to ensure external ancillary scripts work without having to be
	// root or setuid-root
	err = os.Chmod(common.AppPath(p.Root, ra.Name), 0755)
	if err != nil {
		log.Error(err)
		return 254
	}

	workDir := "/"
	if ra.App.WorkingDirectory != "" {
		workDir = ra.App.WorkingDirectory
	}

	rfs := filepath.Join(common.AppPath(p.Root, ra.Name), "rootfs")
	pid := os.Getpid()

	if err = stage1common.WritePid(pid, "pid"); err != nil {
		log.Error(err)
		return 254
	}

	var uidResolver, gidResolver user.Resolver
	var uid, gid int

	uidResolver, err = user.NumericIDs(ra.App.User)
	if err != nil {
		uidResolver, err = user.IDsFromStat(rfs, ra.App.User, nil)
	}

	if err != nil { // give up
		log.PrintE(fmt.Sprintf("invalid user %q", ra.App.User), err)
		return 254
	}

	if uid, _, err = uidResolver.IDs(); err != nil {
		log.PrintE(fmt.Sprintf("failed to configure user %q", ra.App.User), err)
		return 254
	}

	gidResolver, err = user.NumericIDs(ra.App.Group)
	if err != nil {
		gidResolver, err = user.IDsFromStat(rfs, ra.App.Group, nil)
	}

	if err != nil { // give up
		log.PrintE(fmt.Sprintf("invalid group %q", ra.App.Group), err)
		return 254
	}

	if _, gid, err = gidResolver.IDs(); err != nil {
		log.PrintE(fmt.Sprintf("failed to configure group %q", ra.App.Group), err)
		return 254
	}

	diag.Printf("setting uid %d gid %d", uid, gid)
	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		log.PrintE(fmt.Sprintf("can't set gid %d", gid), err)
		return 254
	}

	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		log.PrintE(fmt.Sprintf("can't set uid %d", uid), err)
		return 254
	}

	// Update the runtime path to reflect the absolute path of the container
	path := "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	execDir := filepath.Join(rootDir, rfs)

	var containerPath string
	for _, p := range strings.Split(path, ":") {
		containerPath += execDir + workDir + p + ":"
	}

	env := []string{}

	if fds := os.Getenv("LISTEN_FDS"); fds != "" {
		env = append(env, "LISTEN_FDS="+fds)
	}
	if pid := os.Getenv("LISTEN_PID"); pid != "" {
		env = append(env, "LISTEN_PID="+pid)
	}

	foundPath := false
	for _, e := range ra.App.Environment {
		if e.Name == "PATH" {
			foundPath = true
			for _, p := range strings.Split(e.Value, ":") {
				containerPath += execDir + workDir + p + ":"
			}
			// Add the path for both within the container and on the host as a
			// fallback
			containerPath += e.Value

			env = append(env, e.Name+"="+containerPath)
		} else {
			env = append(env, e.Name+"="+e.Value)
		}
	}

	if !foundPath {
		env = append(env, "PATH="+containerPath+path)
	}

	err = os.Chdir(execDir)
	if err != nil {
		log.PrintE("unable to enter the app rootfs dir", err)
		return 254
	}

	// clear close-on-exec flag on RKT_LOCK_FD, to keep pod status as running after exec().
	if err := sys.CloseOnExec(lfd, false); err != nil {
		log.PrintE("unable to clear FD_CLOEXEC on pod lock", err)
		return 254
	}

	diag.Printf("Starting service: %q\n", filepath.Join(execDir, args[0]))
	if err = syscall.Exec(filepath.Join(execDir, args[0]), args, env); err != nil {
		log.PrintE("cannot exec", err)
		return 254
	}

	return 0
}

func main() {
	rp := parseFlags()

	log, diag, _ = rktlog.NewLogSet("run", debug)
	if !debug {
		diag.SetOutput(ioutil.Discard)
	}

	// move code into stage1() helper so defered fns get run
	os.Exit(stage1(rp))
}
