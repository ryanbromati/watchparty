// Command viewer simula espectadores do watchparty para benchmark:
// login, polling de /api/stats e download dos segmentos novos (como o hls.js).
//
// Uso:
//   viewer -url http://127.0.0.1:PORT -password SENHA -agents 3 -seconds 60
//
// Saída CSV por linha/amostra (~1/s/agent):
//   tMs,agent,hostSeq,segIdx,segFetchMs,statsRttMs,stall
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type stats struct {
	Seq             int `json:"seq"`
	LastMediaEnd    int `json:"lastMediaEnd"`
	BitrateKbps     int `json:"bitrateKbps"`
	SegmentDuration int `json:"segmentDuration"`
	Viewers         int `json:"viewers"`
}

type agent struct {
	id   int
	base *url.URL
	cli  *http.Client
}

func main() {
	base := flag.String("url", "http://127.0.0.1:8080", "URL base do watchparty")
	pw := flag.String("password", "", "senha da sala")
	agentsN := flag.Int("agents", 3, "número de espectadores")
	seconds := flag.Int("seconds", 60, "duração do teste (s)")
	flag.Parse()

	b, err := url.Parse(*base)
	if err != nil {
		fatal(err)
	}

	ags := make([]*agent, 0, *agentsN)
	for i := 0; i < *agentsN; i++ {
		a, err := newAgent(i, b, *pw)
		if err != nil {
			fatal(err)
		}
		ags = append(ags, a)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*seconds)*time.Second)
	defer cancel()

	start := time.Now()
	var wg sync.WaitGroup
	for _, a := range ags {
		wg.Add(1)
		go func(a *agent) {
			defer wg.Done()
			a.run(ctx, start)
		}(a)
	}
	wg.Wait()
}

func newAgent(id int, b *url.URL, pw string) (*agent, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	c := &http.Client{Jar: jar, Timeout: 15 * time.Second}
	a := &agent{id: id, base: b, cli: c}

	body := fmt.Sprintf(`{"password":%s}`, strconv.Quote(pw))
	resp, err := c.Post(b.String()+"/api/login", "application/json", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("login agente %d falhou: %s", id, resp.Status)
	}
	return a, nil
}

func (a *agent) run(ctx context.Context, start time.Time) {
	var lastSeg string
	lastFetch := time.Now()
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		now := time.Now()
		st, rtt, err := a.stats()
		if err != nil {
			fmt.Printf("%d,%d,%d,%s,%d,%d,%d\n", now.Sub(start).Milliseconds(), a.id, -1, lastSeg, -1, rtt, 1)
			continue
		}
		stall := 0
		// simula o player: pega a playlist e baixa o último segmento ainda não baixado.
		names, err := a.latestSegment()
		if err == nil && len(names) > 0 {
			latest := names[len(names)-1]
			if latest != lastSeg {
				fetchMs := a.download(latest)
				if fetchMs >= 0 {
					lastSeg = latest
					lastFetch = now
					idx := parseIdx(latest)
					fmt.Printf("%d,%d,%d,%d,%d,%d,%d\n", now.Sub(start).Milliseconds(), a.id, st.Seq, idx, fetchMs, rtt, 0)
					continue
				}
				// falha ao baixar o segmento novo = stall
				stall = 1
			} else if now.Sub(lastFetch) > 4*time.Second {
				stall = 1
			}
		}
		fmt.Printf("%d,%d,%d,%s,%d,%d,%d\n", now.Sub(start).Milliseconds(), a.id, st.Seq, lastSeg, -1, rtt, stall)
	}
}

func (a *agent) stats() (stats, int64, error) {
	t0 := time.Now()
	resp, err := a.cli.Get(a.base.String() + "/api/stats")
	rtt := time.Since(t0).Milliseconds()
	if err != nil {
		return stats{}, rtt, err
	}
	defer resp.Body.Close()
	var st stats
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return st, rtt, fmt.Errorf("stats status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return st, rtt, err
	}
	return st, rtt, nil
}

// latestSegment baixa a playlist e devolve os nomes dos segmentos listados.
func (a *agent) latestSegment() ([]string, error) {
	resp, err := a.cli.Get(a.base.String() + "/live/index.m3u8")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("playlist status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		names = append(names, line)
	}
	return names, nil
}

func (a *agent) download(name string) int64 {
	u := a.base.String() + "/live/" + name
	t0 := time.Now()
	resp, err := a.cli.Get(u)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return -1
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return time.Since(t0).Milliseconds()
}

func parseIdx(name string) int {
	trim := strings.TrimSuffix(strings.TrimPrefix(name, "seg_"), ".ts")
	v, _ := strconv.Atoi(trim)
	return v
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "erro:", err)
	os.Exit(1)
}
