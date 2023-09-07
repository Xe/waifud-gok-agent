package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/exp/slog"
	"gopkg.in/yaml.v3"
)

var (
	slogLevel = flag.String("slog-level", "INFO", "log level")

	serialRegex = regexp.MustCompile(`^ds=([a-zA-Z0-9-_]+);s=(https?://([a-zA-Z0-9-.:]+)/api/cloudinit/[a-f0-9-]+/)$`)
)

func main() {
	var programLevel slog.Level
	if err := (&programLevel).UnmarshalText([]byte(*slogLevel)); err != nil {
		fmt.Fprintf(os.Stderr, "invalid log level %s: %v, using info\n", *slogLevel, err)
		programLevel = slog.LevelInfo
	}

	leveler := &slog.LevelVar{}
	leveler.Set(programLevel)

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     leveler,
	})
	slog.SetDefault(slog.New(h))

	data, err := os.ReadFile("/sys/devices/virtual/dmi/id/product_serial")
	if err != nil {
		log.Fatalf("can't read serial number: %v", err)
	}

	sp := serialRegex.FindStringSubmatch(string(data))

	slog.Info("got splits", "sp", sp)

	url := sp[2] + "meta-data"

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		io.Copy(os.Stderr, resp.Body)
		log.Fatal(err)
	}
	defer resp.Body.Close()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var metadata Metadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		log.Fatal(err)
	}

	os.Remove("/etc/machine-id")
	if err := os.WriteFile("/etc/machine-id", []byte(strings.ReplaceAll(metadata.InstanceID, "-", "")), 444); err != nil {
		log.Fatal(err)
	}

	os.Remove("/etc/hostname")
	if err := os.WriteFile("/etc/hostname", []byte(metadata.Hostname), 444); err != nil {
		log.Fatal(err)
	}
	if err := syscall.Sethostname([]byte(metadata.Hostname)); err != nil {
		log.Fatal(err)
	}
}

type Metadata struct {
	InstanceID string `yaml:"instance-id"`
	Hostname   string `yaml:"hostname"`
}
