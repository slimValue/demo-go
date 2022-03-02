package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"time"

	. "demo-pprof/http_pprof/feat"
)

func doHTTPRequest(url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	data, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("ret:", len(data))
	resp.Body.Close()
}


func main() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			doHTTPRequest(fmt.Sprintf("http://localhost:8080/fib/%d", rand.Intn(30)))
			time.Sleep(500 * time.Millisecond)
		}
	}()

	go func() {
		defer wg.Done()
		for {
			doHTTPRequest(fmt.Sprintf("http://localhost:8080/repeat/%s/%d", Generate(rand.Intn(200)), rand.Intn(200)))
			time.Sleep(500 * time.Millisecond)
		}
	}()
	wg.Wait()
}