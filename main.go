package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/google/go-github/v64/github"
	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/inserter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
)

var (
	githubClient *github.Client
	httpClient   = &http.Client{
		Timeout: 10 * time.Minute,
	}
)

var providerRuleSets = map[string]string{
	"cloudflare": "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/srs/cloudflare.srs",
	"cloudfront": "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/srs/cloudfront.srs",
	"facebook":   "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/srs/facebook.srs",
	"fastly":     "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/srs/fastly.srs",
	"google":     "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/srs/google.srs",
	"netflix":    "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/srs/netflix.srs",
	"telegram":   "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/srs/telegram.srs",
	"tor":        "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/srs/tor.srs",
	"twitter":    "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/srs/twitter.srs",
}

func init() {
	accessToken, loaded := os.LookupEnv("ACCESS_TOKEN")
	if !loaded {
		githubClient = github.NewClient(nil)
		return
	}
	transport := &github.BasicAuthTransport{
		Username: accessToken,
	}
	githubClient = github.NewClient(transport.Client())
}

func fetch(from string) (*github.RepositoryRelease, error) {
	fixedRelease := os.Getenv("FIXED_RELEASE")
	names := strings.SplitN(from, "/", 2)
	if len(names) != 2 || names[0] == "" || names[1] == "" {
		return nil, E.New("invalid GitHub repository: ", from)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if fixedRelease != "" {
		latestRelease, _, err := githubClient.Repositories.GetReleaseByTag(ctx, names[0], names[1], fixedRelease)
		if err != nil {
			return nil, err
		}
		return latestRelease, err
	} else {
		latestRelease, _, err := githubClient.Repositories.GetLatestRelease(ctx, names[0], names[1])
		if err != nil {
			return nil, err
		}
		return latestRelease, err
	}
}

func get(downloadURL *string) ([]byte, error) {
	log.Info("download ", *downloadURL)
	var lastErr error
	for i := 0; i < 3; i++ {
		response, err := httpClient.Get(*downloadURL)
		if err != nil {
			lastErr = err
			log.Warn("download attempt ", i+1, " failed: ", err)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		if response.StatusCode != http.StatusOK {
			lastErr = E.New("download ", *downloadURL, " failed with status ", response.Status)
			response.Body.Close()
			log.Warn("download attempt ", i+1, " failed: ", lastErr)
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
			continue
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			lastErr = readErr
			log.Warn("download attempt ", i+1, " failed: ", readErr)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		return body, nil
	}
	return nil, lastErr
}

func download(release *github.RepositoryRelease) ([]byte, error) {
	geoipAsset := common.Find(release.Assets, func(it *github.ReleaseAsset) bool {
		return *it.Name == "Country.mmdb"
	})
	if geoipAsset == nil {
		return nil, E.New("Country.mmdb not found in upstream release ", release.Name)
	}
	return get(geoipAsset.BrowserDownloadURL)
}

func downloadProviderRuleSets(output string) error {
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	names := make([]string, 0, len(providerRuleSets))
	for name := range providerRuleSets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		url := providerRuleSets[name]
		data, err := get(&url)
		if err != nil {
			return E.New("download provider rule set ", name, ": ", err)
		}
		if !bytes.HasPrefix(data, []byte("SRS\x01")) {
			return E.New("invalid provider rule set ", name)
		}
		path := filepath.Join(output, "provider-"+name+".srs")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		log.Info("write ", path)
	}
	return nil
}

func parse(binary []byte) (metadata maxminddb.Metadata, countryMap map[string][]*net.IPNet, err error) {
	database, err := maxminddb.FromBytes(binary)
	if err != nil {
		return
	}
	metadata = database.Metadata
	networks := database.Networks(maxminddb.SkipAliasedNetworks)
	countryMap = make(map[string][]*net.IPNet)
	var country geoip2.Enterprise
	var ipNet *net.IPNet
	for networks.Next() {
		ipNet, err = networks.Network(&country)
		if err != nil {
			return
		}
		// Country codes must be lowercase per sing-box convention.
		code := strings.ToLower(country.RegisteredCountry.IsoCode)
		if code == "" {
			continue
		}
		countryMap[code] = append(countryMap[code], ipNet)
	}
	err = networks.Err()
	return
}

func newWriter(metadata maxminddb.Metadata, codes []string) (*mmdbwriter.Tree, error) {
	return mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "sing-geoip",
		Languages:               codes,
		IPVersion:               int(metadata.IPVersion),
		RecordSize:              int(metadata.RecordSize),
		Inserter:                inserter.ReplaceWith,
		DisableIPv4Aliasing:     true,
		IncludeReservedNetworks: true,
	})
}

func write(writer *mmdbwriter.Tree, dataMap map[string][]*net.IPNet, output string, codes []string) error {
	if len(codes) == 0 {
		codes = make([]string, 0, len(dataMap))
		for code := range dataMap {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	codeMap := make(map[string]bool)
	for _, code := range codes {
		codeMap[code] = true
	}
	for code, data := range dataMap {
		if !codeMap[code] {
			continue
		}
		for _, item := range data {
			err := writer.Insert(item, mmdbtype.String(code))
			if err != nil {
				return err
			}
		}
	}
	outputFile, err := os.Create(output)
	if err != nil {
		return err
	}
	defer outputFile.Close()
	_, err = writer.WriteTo(outputFile)
	return err
}

func release(source string, destination string, output string, ruleSetOutput string) error {
	sourceRelease, err := fetch(source)
	if err != nil {
		return err
	}
	destinationRelease, err := fetch(destination)
	if err != nil {
		log.Warn("missing destination latest release")
	} else {
		if os.Getenv("NO_SKIP") != "true" && strings.Contains(*destinationRelease.Name, *sourceRelease.Name) {
			log.Info("already latest")
			setActionOutput("skip", "true")
			return nil
		}
	}
	binary, err := download(sourceRelease)
	if err != nil {
		return err
	}
	metadata, countryMap, err := parse(binary)
	if err != nil {
		return err
	}
	allCodes := make([]string, 0, len(countryMap))
	for code := range countryMap {
		allCodes = append(allCodes, code)
	}

	writer, err := newWriter(metadata, allCodes)
	if err != nil {
		return err
	}
	err = write(writer, countryMap, output, nil)
	if err != nil {
		return err
	}

	writer, err = newWriter(metadata, []string{"id"})
	if err != nil {
		return err
	}
	err = write(writer, countryMap, "geoip-id.db", []string{"id"})
	if err != nil {
		return err
	}

	os.RemoveAll(ruleSetOutput)
	err = os.MkdirAll(ruleSetOutput, 0o755)
	if err != nil {
		return err
	}
	for countryCode, ipNets := range countryMap {
		var headlessRule option.DefaultHeadlessRule
		headlessRule.IPCIDR = make([]string, 0, len(ipNets))
		for _, cidr := range ipNets {
			headlessRule.IPCIDR = append(headlessRule.IPCIDR, cidr.String())
		}
		var plainRuleSet option.PlainRuleSet
		plainRuleSet.Rules = []option.HeadlessRule{
			{
				Type:           C.RuleTypeDefault,
				DefaultOptions: headlessRule,
			},
		}
		srsPath, _ := filepath.Abs(filepath.Join(ruleSetOutput, "geoip-"+countryCode+".srs"))
		log.Info("write ", srsPath)
		outputRuleSet, err := os.Create(srsPath)
		if err != nil {
			return err
		}
		err = srs.Write(outputRuleSet, plainRuleSet, C.RuleSetVersionCurrent)
		if err != nil {
			outputRuleSet.Close()
			return err
		}
		outputRuleSet.Close()
	}
	if err := downloadProviderRuleSets(ruleSetOutput); err != nil {
		return err
	}

	setActionOutput("tag", *sourceRelease.Name)
	return nil
}

func setActionOutput(name string, content string) {
	ghOutput := os.Getenv("GITHUB_OUTPUT")
	if ghOutput == "" {
		log.Warn("GITHUB_OUTPUT not set, skipping output: ", name)
		return
	}
	f, err := os.OpenFile(ghOutput, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Warn("failed to open GITHUB_OUTPUT: ", err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s=%s\n", name, content)
}

func main() {
	err := release("Dreamacro/maxmind-geoip", "bitscoid/BITS-GeoIP", "geoip.db", "rule-set")
	if err != nil {
		log.Fatal(err)
	}
}
