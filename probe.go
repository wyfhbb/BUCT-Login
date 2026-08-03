package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// defaultProbeURLs is the site pool a run rotates through. They are the captive
// portal detection endpoints Chinese vendors ship in their phones: they answer
// 204 with an empty body, they are hosted in mainland China, and they exist to
// be polled, so probing them costs both sides next to nothing.
//
// Anything else answering in their place (a redirect, a login page) is the
// campus portal intercepting the request, which is exactly what tells us the
// session is gone without ever asking the portal itself.
var defaultProbeURLs = []string{
	"http://connect.rom.miui.com/generate_204",                   // 小米
	"http://connectivitycheck.platform.hicloud.com/generate_204", // 华为
	"http://wifi.vivo.com.cn/generate_204",                       // vivo
	"http://www.qualcomm.cn/generate_204",                        // 高通中国
	"http://204.ustclug.org/",                                    // 中科大 LUG
}

const probeUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// probeResult tells apart the two ways a probe can fail, because they mean
// different things: an intercepted request is proof enough that the portal is
// in the way, while a target that stays silent may simply be down.
type probeResult int

const (
	probeOK          probeResult = iota // the target answered the way it should
	probeIntercepted                    // something answered in its place
	probeUnreachable                    // nothing answered at all
)

// newProbeClient builds the client used for the pool
func newProbeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		// The probe has to describe this machine's own path to the internet, a
		// proxy from the environment would hide exactly what is being measured
		Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true},
		// The portal answers with a redirect to its login page, following it
		// would report the login page instead of the interception
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// expects204 reports whether the target is a detection endpoint. Those are
// recognised by the 204 in their address, which every such endpoint carries.
func expects204(target string) bool {
	return strings.Contains(target, "204")
}

// isProbeSuccess reports whether the answer really came from the target
func isProbeSuccess(target string, resp *http.Response) bool {
	if expects204(target) {
		// An empty 200 is accepted too, a few mirrors answer that way
		return resp.StatusCode == http.StatusNoContent ||
			(resp.StatusCode == http.StatusOK && resp.ContentLength == 0)
	}

	// A custom target can only be judged by its status: a 2xx is the site
	// itself, anything else is either the portal or a broken site, and both
	// are a reason to go and ask the portal
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// probeTarget performs a single probe
func probeTarget(client *http.Client, target string) (probeResult, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return probeUnreachable, err
	}
	req.Header.Set("User-Agent", probeUserAgent)
	// A cached answer would say nothing about the state of the link right now
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return probeUnreachable, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if isProbeSuccess(target, resp) {
		return probeOK, nil
	}

	return probeIntercepted, fmt.Errorf("HTTP %d", resp.StatusCode)
}

// probeURLs returns the pool to use, falling back to the built-in one
func probeURLs(config *Config) []string {
	if len(config.ProbeURLs) > 0 {
		return config.ProbeURLs
	}
	return defaultProbeURLs
}

// runProbeBudget returns the worst case time one probe cycle can take, which is
// what the unit timeout and the interval hint have to account for
func runProbeBudget(config *Config) int {
	if !config.ProbeEnabled {
		return 0
	}

	attempts := config.ProbeFailThreshold
	if attempts < 1 {
		attempts = 1
	}
	if pool := len(probeURLs(config)); attempts > pool {
		attempts = pool
	}

	timeout := config.ProbeTimeout
	if timeout < 1 {
		timeout = 1
	}

	return attempts * timeout
}

// runProbes walks the pool from where the previous run stopped and reports
// whether the portal has to be asked about the session.
//
// It stops at the first target that answers, and at the first one that is
// clearly intercepted. A target that stays silent is not trusted on its own:
// only after ProbeFailThreshold of them in a row is the network considered
// down, so a single site having a bad day costs nothing.
//
// state is advanced past every target that was tried, so consecutive runs
// spread their requests over the whole pool instead of hammering one site.
func runProbes(config *Config, state *State) (bool, string) {
	urls := probeURLs(config)

	attempts := config.ProbeFailThreshold
	if attempts < 1 {
		attempts = 1
	}
	if attempts > len(urls) {
		attempts = len(urls)
	}

	timeout := config.ProbeTimeout
	if timeout < 1 {
		timeout = 1
	}
	client := newProbeClient(time.Duration(timeout) * time.Second)

	start := state.ProbeIndex
	if start < 0 || start >= len(urls) {
		start = 0
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		target := urls[(start+i)%len(urls)]
		result, err := probeTarget(client, target)
		state.ProbeIndex = (start + i + 1) % len(urls)

		switch result {
		case probeOK:
			logrus.Infof("Probe %s: reachable", target)
			return false, target
		case probeIntercepted:
			logrus.Warnf("Probe %s: intercepted (%v)", target, err)
			return true, fmt.Sprintf("%s intercepted: %v", target, err)
		default:
			logrus.Warnf("Probe %s: no answer (%v)", target, err)
			lastErr = err
		}
	}

	return true, fmt.Sprintf("%d targets in a row gave no answer, last error: %v", attempts, lastErr)
}
