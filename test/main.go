package main

import "time"

func main() {
	var ch = make(chan struct{}, 1)
	defer func() {
		time.After(1 *time.Second)
		close(ch)
		println("is closing")
	}()

	go func() {
		<-ch
		println("has closed")
	}()

	//go func() {
	//	time.After(1 *time.Second)
	//	close(ch)
	//	println("is closing")
	//}()

	println(1 << 1)
	println(1 << 8)
	println(1 << 4)
	println(1 << 2)
	println("start")

	time.Sleep(2*time.Second)
}
