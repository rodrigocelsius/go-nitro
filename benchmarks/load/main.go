package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9000", "")
	path := flag.String("path", "/ping", "")
	method := flag.String("m", "GET", "")
	body := flag.String("body", "", "")
	conns := flag.Int("c", 64, "")
	dur := flag.Duration("d", 8*time.Second, "")
	flag.Parse()
	req := *method + " " + *path + " HTTP/1.1\r\nHost: x\r\n"
	if *body != "" {
		req += "Content-Type: application/json\r\nContent-Length: " + strconv.Itoa(len(*body)) + "\r\n"
	}
	req += "\r\n" + *body
	var ok, bad int64
	var mu sync.Mutex
	var lats []time.Duration
	stop := time.Now().Add(*dur)
	var wg sync.WaitGroup
	for i := 0; i < *conns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := net.Dial("tcp", *addr)
			if err != nil {
				atomic.AddInt64(&bad, 1)
				return
			}
			defer c.Close()
			br := bufio.NewReaderSize(c, 8192)
			local := make([]time.Duration, 0, 1<<16)
			for time.Now().Before(stop) {
				t := time.Now()
				if _, err := c.Write([]byte(req)); err != nil {
					atomic.AddInt64(&bad, 1)
					return
				}
				line, err := br.ReadString('\n')
				if err != nil {
					atomic.AddInt64(&bad, 1)
					return
				}
				good := strings.Contains(line, " 200")
				cl := -1
				chunked := false
				for {
					h, err := br.ReadString('\n')
					if err != nil {
						atomic.AddInt64(&bad, 1)
						return
					}
					if h == "\r\n" {
						break
					}
					l := strings.ToLower(h)
					if strings.HasPrefix(l, "content-length:") {
						cl, _ = strconv.Atoi(strings.TrimSpace(h[15:]))
					}
					if strings.HasPrefix(l, "transfer-encoding: chunked") {
						chunked = true
					}
				}
				if chunked {
					for {
						s, _ := br.ReadString('\n')
						n, _ := strconv.ParseInt(strings.TrimSpace(s), 16, 64)
						br.Discard(int(n) + 2)
						if n == 0 {
							break
						}
					}
				} else if cl > 0 {
					br.Discard(cl)
				}
				if good {
					atomic.AddInt64(&ok, 1)
				} else {
					atomic.AddInt64(&bad, 1)
				}
				if len(local) < cap(local) {
					local = append(local, time.Since(t))
				}
			}
			mu.Lock()
			lats = append(lats, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	p := func(q float64) time.Duration {
		if len(lats) == 0 {
			return 0
		}
		return lats[int(float64(len(lats)-1)*q)]
	}
	fmt.Printf("%.0f %d %v %v %v\n", float64(ok)/dur.Seconds(), bad, p(.5), p(.99), p(.999))
}
