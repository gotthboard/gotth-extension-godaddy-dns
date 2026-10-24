package main

import (
	"bufio"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const modulePath = "github.com/gotthboard/gotth-extension-godaddy-dns"

type inventoryEntry struct {
	Path, Version, Sum, LicenseID string
}

type document struct {
	SPDXVersion       string         `json:"spdxVersion"`
	DataLicense       string         `json:"dataLicense"`
	SPDXID            string         `json:"SPDXID"`
	Name              string         `json:"name"`
	DocumentNamespace string         `json:"documentNamespace"`
	CreationInfo      creationInfo   `json:"creationInfo"`
	DocumentDescribes []string       `json:"documentDescribes"`
	Packages          []spdxPackage  `json:"packages"`
	Relationships     []relationship `json:"relationships"`
}

type creationInfo struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}

type spdxPackage struct {
	Name             string        `json:"name"`
	SPDXID           string        `json:"SPDXID"`
	VersionInfo      string        `json:"versionInfo"`
	PackageFileName  string        `json:"packageFileName,omitempty"`
	DownloadLocation string        `json:"downloadLocation"`
	FilesAnalyzed    bool          `json:"filesAnalyzed"`
	LicenseConcluded string        `json:"licenseConcluded"`
	LicenseDeclared  string        `json:"licenseDeclared"`
	CopyrightText    string        `json:"copyrightText"`
	Checksums        []checksum    `json:"checksums,omitempty"`
	ExternalRefs     []externalRef `json:"externalRefs,omitempty"`
	SourceInfo       string        `json:"sourceInfo,omitempty"`
}

type checksum struct {
	Algorithm     string `json:"algorithm"`
	ChecksumValue string `json:"checksumValue"`
}

type externalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

type relationship struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "generate SPDX SBOM:", err)
		os.Exit(1)
	}
}

// Complexity: time O(n+m+b+s), Omega(n+m+b), with no single tight bound
// because executable metadata scanning depends on format; auxiliary space O(n+m),
// Omega(n+m), tight Theta(n+m), where n is inventory bytes, m is linked
// modules, b is archive bytes, and s is executable bytes inspected.
func run(args []string) error {
	if len(args) != 5 {
		return errors.New("usage: <binary> <archive> <version> <dependencies.tsv> <output>")
	}
	binaryPath, archivePath, version, inventoryPath, outputPath := args[0], args[1], args[2], args[3], args[4]
	if version != "1.0.0" {
		return errors.New("release version must be 1.0.0")
	}
	epoch, err := strconv.ParseInt(os.Getenv("SOURCE_DATE_EPOCH"), 10, 64)
	if err != nil || epoch < 1 {
		return errors.New("valid SOURCE_DATE_EPOCH required")
	}
	inventory, err := readInventory(inventoryPath)
	if err != nil {
		return err
	}
	info, err := buildinfo.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("read binary build info: %w", err)
	}
	if info.Path != modulePath+"/cmd/gotth-extension-godaddy-dns" {
		return fmt.Errorf("unexpected binary package %q", info.Path)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	revision := settings["vcs.revision"]
	if len(revision) != 40 || settings["vcs"] != "git" || settings["vcs.modified"] != "true" && settings["vcs.modified"] != "false" {
		return errors.New("complete Git source identity missing from binary")
	}
	linked := make(map[string]*inventoryEntry, len(info.Deps))
	for _, dep := range info.Deps {
		if dep.Replace != nil {
			return fmt.Errorf("replacement forbidden for %s", dep.Path)
		}
		want, ok := inventory[dep.Path]
		if !ok || dep.Version != want.Version || dep.Sum != want.Sum {
			return fmt.Errorf("unadmitted linked module %s %s %s", dep.Path, dep.Version, dep.Sum)
		}
		linked[dep.Path] = want
	}
	if len(linked) != len(inventory) {
		return fmt.Errorf("inventory has %d modules but binary links %d", len(inventory), len(linked))
	}
	archiveSum, err := fileSHA256(archivePath)
	if err != nil {
		return err
	}
	rootID := "SPDXRef-Package-gotth-extension-godaddy-dns"
	doc := document{
		SPDXVersion: "SPDX-2.3", DataLicense: "CC0-1.0", SPDXID: "SPDXRef-DOCUMENT",
		Name:              "gotth-extension-godaddy-dns-1.0.0-linux-amd64",
		DocumentNamespace: "urn:gotth:sbom:gotth-extension-godaddy-dns:v1.0.0:" + archiveSum,
		CreationInfo:      creationInfo{Created: time.Unix(epoch, 0).UTC().Format(time.RFC3339), Creators: []string{"Tool: gotth-extension-godaddy-dns-sbom/1.0.0"}},
		DocumentDescribes: []string{rootID},
	}
	doc.Packages = append(doc.Packages, spdxPackage{
		Name: "gotth-extension-godaddy-dns", SPDXID: rootID, VersionInfo: version, PackageFileName: archivePath[strings.LastIndex(archivePath, "/")+1:],
		DownloadLocation: "NOASSERTION", FilesAnalyzed: false, LicenseConcluded: "MIT", LicenseDeclared: "MIT", CopyrightText: "Copyright (c) 2026 GOTTH Board contributors",
		Checksums:    []checksum{{Algorithm: "SHA256", ChecksumValue: archiveSum}},
		ExternalRefs: []externalRef{{ReferenceCategory: "PACKAGE-MANAGER", ReferenceType: "purl", ReferenceLocator: "pkg:golang/" + modulePath + "@v" + version}},
		SourceInfo:   "Git revision " + revision + "; vcs.modified=" + settings["vcs.modified"],
	})
	paths := make([]string, 0, len(inventory))
	for path := range inventory {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		entry := inventory[path]
		id := spdxID(path)
		doc.Packages = append(doc.Packages, spdxPackage{Name: path, SPDXID: id, VersionInfo: entry.Version, DownloadLocation: "NOASSERTION", FilesAnalyzed: false, LicenseConcluded: entry.LicenseID, LicenseDeclared: entry.LicenseID, CopyrightText: "NOASSERTION", ExternalRefs: []externalRef{{ReferenceCategory: "PACKAGE-MANAGER", ReferenceType: "purl", ReferenceLocator: "pkg:golang/" + path + "@" + entry.Version}}, SourceInfo: "Go module sum " + entry.Sum})
		doc.Relationships = append(doc.Relationships, relationship{SPDXElementID: rootID, RelationshipType: "DEPENDS_ON", RelatedSPDXElement: id})
	}
	doc.Relationships = append(doc.Relationships, relationship{SPDXElementID: "SPDXRef-DOCUMENT", RelationshipType: "DESCRIBES", RelatedSPDXElement: rootID})
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
	err = encoder.Encode(doc)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	complete = true
	return nil
}

// Complexity: time O(n), Omega(n), tight Theta(n); auxiliary space O(n), Omega(n), tight Theta(n), where n is the inventory byte length.
func readInventory(path string) (map[string]*inventoryEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries := map[string]*inventoryEntry{}
	scanner := bufio.NewScanner(io.LimitReader(f, 64<<10))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 6 || fields[0] == "" || fields[1] == "" || fields[2] == "" || fields[3] == "" {
			return nil, errors.New("invalid dependency inventory")
		}
		if _, exists := entries[fields[0]]; exists {
			return nil, errors.New("duplicate dependency inventory entry")
		}
		entries[fields[0]] = &inventoryEntry{Path: fields[0], Version: fields[1], Sum: fields[2], LicenseID: fields[3]}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, errors.New("empty dependency inventory")
	}
	return entries, nil
}

// Complexity: time O(n), Omega(n), tight Theta(n); auxiliary space O(1), Omega(1), tight Theta(1), where n is the file byte length.
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
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Complexity: time O(n), Omega(n), tight Theta(n); auxiliary space O(n), Omega(n), tight Theta(n), where n is the module-path rune count.
func spdxID(path string) string {
	return "SPDXRef-Package-" + strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '.' {
			return r
		}
		return '-'
	}, path)
}
