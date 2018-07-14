package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/gokrazy/internal/fat"
	"github.com/gokrazy/internal/rootdev"
)

var (
	listen = flag.String("listen",
		":1341",
		"listen address")

	bootUnix = flag.Int64("boot_unix", 0, "do not use")
	rootUnix = flag.Int64("root_unix", 0, "do not use")
)

func getBootTimestamp() (time.Time, error) {
	f, err := os.OpenFile(rootdev.MustFind()+"1", os.O_RDONLY, 0600)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()

	rd, err := fat.NewReader(f)
	if err != nil {
		return time.Time{}, err
	}
	return rd.ModTime("/cmdline.txt")
}

func getRootTimestamp() (time.Time, error) {
	st, err := os.Stat("/etc/hostname")
	if err != nil && os.IsNotExist(err) {
		st, err = os.Stat("/hostname")
	}
	if err != nil {
		return time.Time{}, err
	}
	return st.ModTime(), nil
}

// mustDropPrivileges executes the program in a child process, dropping root
// privileges.
func mustDropPrivileges(bootT, rootT time.Time) {
	cmd := exec.Command(os.Args[0],
		fmt.Sprintf("-boot_unix=%d", bootT.Unix()),
		fmt.Sprintf("-root_unix=%d", rootT.Unix()))
	cmd.Env = append(os.Environ(), "TIMESTAMPS_PRIVILEGES_DROPPED=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: 65534,
			Gid: 65534,
		},
	}
	log.Fatal(cmd.Run())
}

func main() {
	flag.Parse()

	if os.Getenv("TIMESTAMPS_PRIVILEGES_DROPPED") != "1" {
		bootT, err := getBootTimestamp()
		if err != nil {
			log.Fatal(err)
		}
		rootT, err := getRootTimestamp()
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("timestamps: boot: %v, root: %v", bootT, rootT)

		mustDropPrivileges(bootT, rootT)
	}

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// As per https://prometheus.io/docs/instrumenting/exposition_formats/
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "boot_build_timestamp %d\n", *bootUnix)
		fmt.Fprintf(w, "root_build_timestamp %d\n", *rootUnix)
	})
	log.Fatal(http.ListenAndServe(*listen, nil))
}
