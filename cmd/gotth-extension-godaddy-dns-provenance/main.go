package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const modulePath = "github.com/gotthboard/gotth-extension-godaddy-dns"

type digest struct {
	SHA256 string `json:"sha256"`
}

type subject struct {
	Name   string `json:"name"`
	Digest digest `json:"digest"`
}

type source struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Tree       string `json:"tree"`
	Modified   bool   `json:"modified"`
}

type toolchain struct {
	GoVersion      string `json:"goVersion"`
	GoExperiment   string `json:"goExperiment"`
	GoBinarySHA256 string `json:"goBinarySha256"`
}

type parameters struct {
	Version         string `json:"version"`
	TargetOS        string `json:"targetOs"`
	TargetArch      string `json:"targetArch"`
	GOAMD64         string `json:"goAmd64"`
	CGOEnabled      string `json:"cgoEnabled"`
	Trimpath        bool   `json:"trimpath"`
	BuildVCS        bool   `json:"buildVcs"`
	LDFlags         string `json:"ldflags"`
	SourceDateEpoch int64  `json:"sourceDateEpoch"`
}

type provenance struct {
	Schema     string     `json:"schema"`
	Subject    []subject  `json:"subject"`
	Source     source     `json:"source"`
	Toolchain  toolchain  `json:"toolchain"`
	Parameters parameters `json:"parameters"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "generate provenance:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 8 {
		return errors.New("usage: <binary> <archive> <sbom> <version> <commit> <tree> <modified> <output>")
	}
	binaryPath, archivePath, sbomPath, version := args[0], args[1], args[2], args[3]
	commit, tree, modifiedText, outputPath := args[4], args[5], args[6], args[7]
	if version != "1.0.0" || len(commit) != 40 || len(tree) != 40 {
		return errors.New("exact v1.0.0 source identity required")
	}
	modified, err := strconv.ParseBool(modifiedText)
	if err != nil {
		return errors.New("valid modified state required")
	}
	epoch, err := strconv.ParseInt(os.Getenv("SOURCE_DATE_EPOCH"), 10, 64)
	if err != nil || epoch < 1 {
		return errors.New("valid SOURCE_DATE_EPOCH required")
	}
	goBinarySHA256 := os.Getenv("GOTTH_GO_BINARY_SHA256")
	if len(goBinarySHA256) != 64 {
		return errors.New("exact Go binary SHA-256 required")
	}
	info, err := buildinfo.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("read binary build info: %w", err)
	}
	if info.Path != modulePath+"/cmd/gotth-extension-godaddy-dns" {
		return errors.New("unexpected binary package")
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["vcs.revision"] != commit || settings["vcs.modified"] != modifiedText || settings["GOOS"] != "linux" || settings["GOARCH"] != "amd64" || settings["CGO_ENABLED"] != "0" || settings["-trimpath"] != "true" {
		return errors.New("binary build identity does not match provenance")
	}
	archiveSum, err := fileSHA256(archivePath)
	if err != nil {
		return err
	}
	sbomSum, err := fileSHA256(sbomPath)
	if err != nil {
		return err
	}
	doc := provenance{
		Schema:     "urn:gotth:release-provenance:v1",
		Subject:    []subject{{Name: filepath.Base(archivePath), Digest: digest{SHA256: archiveSum}}, {Name: filepath.Base(sbomPath), Digest: digest{SHA256: sbomSum}}},
		Source:     source{Repository: "urn:gotth:source:gotthboard/gotth-extension-godaddy-dns", Commit: commit, Tree: tree, Modified: modified},
		Toolchain:  toolchain{GoVersion: info.GoVersion, GoExperiment: settings["GOEXPERIMENT"], GoBinarySHA256: goBinarySHA256},
		Parameters: parameters{Version: "v1.0.0", TargetOS: "linux", TargetArch: "amd64", GOAMD64: settings["GOAMD64"], CGOEnabled: "0", Trimpath: true, BuildVCS: true, LDFlags: "-s -w -buildid= -X " + modulePath + "/internal/godaddydns.BuildVersion=1.0.0", SourceDateEpoch: epoch},
	}
	if doc.Toolchain.GoVersion == "" || doc.Toolchain.GoExperiment == "" || doc.Parameters.GOAMD64 == "" {
		return errors.New("incomplete toolchain provenance")
	}
	f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(outputPath)
		}
	}()
	encoder := json.NewEncoder(f)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(doc)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	complete = true
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return strings.ToLower(hex.EncodeToString(h.Sum(nil))), nil
}
