package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
	"syscall"
)
var (
	src      *os.File
	size     int64
	headers  string
	offsetsz int     = 4096
	offset   []int64 = make([]int64, offsetsz, offsetsz)
	srcfd    int
	mutex    sync.Mutex
)
const (
	P_DEFAULT_HOST      = "127.0.0.1"
	P_DEFAULT_PORT      = "8080"
	P_DEFAULT_MIME_TYPE = "text/plain"
	P_HEAD_TMPL         = "HTTP/1.0 200 OK\r\nCache-Control: max-age=31536000\r\nExpires: Thu, 31 Dec 2037 23:55:55 GMT\r\nContent-Type: {{.Mime}}\r\nContent-Length: {{.Length}}\r\n\r\n"
)
func main() {
	host, port, mimetype, procs := parseArgs()
	runtime.GOMAXPROCS(procs)
	fmt.Printf("监听的地址：端口 %s:%s\n", host, port)
	log.Println("设置的核心数为： ", procs)
	fmt.Printf("描述消息内容类型为： %s\n", mimetype)
	addr := host + ":" + port
	sock, lerr := net.Listen("tcp", addr)
	if lerr != nil {
		log.Fatal("网关启动失败 ", addr, ". ", lerr)
	}
	for {
		conn, aerr := sock.Accept()
		if aerr != nil {
			log.Fatal("Error Accept. ", aerr)
		}
		// 另外起一个协程去处理这个事，这里推荐用携程池，任何用到协程的地方都要特别注意协程的数量
		go handle(conn)
	}
}
func handle(conn net.Conn) {
	log.Println("开始调用转发······")
	// 我需要转发到下面这个地址当中
	cli_conn, err := net.Dial("tcp", "127.0.0.1:10000")
	if err != nil {
		log.Println("connect error",err)
		return
	}
	defer cli_conn.Close()
	srcfile, ferr := conn.(*net.TCPConn).File()
	outfile, err := cli_conn.(*net.TCPConn).File()
	if ferr != nil {
		log.Fatal("TCP连接拿到的文件描述符错误：", ferr)
	}
	srcfd := int(srcfile.Fd())
	outfd := int(outfile.Fd())
	if srcfd >= offsetsz {
		growOffset(srcfd)
	}
	currOffset := &offset[srcfd]
	// 零拷贝我是接收到了一个连接，可是我采用syscall.Sendfile()，我怎么转发这个端口？？？？
	// 思考1：我如果read出来，那么就会把数据读取了，那这样 网卡=》应用缓冲区
	// sendfile有四个参数：outfd int, infd int, offset *int64, count int
	//outfd是带读出内容的文件描述符、infd是待写入的内容的文件描述符、
	//offset是指定从文件流的哪个位置开始读（为空默认从头开始读）、count参数指定文件描述符in_fd和out_fd之间传输的字节数
	// in_fd必须是一个支持类似mmap函数的文件描述符（也就是必须指向真实文件）、out_fd是一个socket
	for *currOffset < size {
		// 需要解决的一个问题就是从哪去得到这个需要发送的目标socket缓存区，由这个位置读取到网卡进一步转发，cli_conn怎么去拿outfd
		_, werr := syscall.Sendfile(outfd, srcfd, currOffset, int(size))
		if werr != nil {
			log.Fatal("系统调用Sendfile发送错误:", werr)
		}
	}
}
func growOffset(outfd int) {
	//  只允许一个协程来增长切片的偏移，否则会造成数据混乱
	mutex.Lock()
	// 加多一层校验，判断是否还需要这样去增长偏移（可能其他协程已经做完离开了）
	if outfd < offsetsz {
		mutex.Unlock()
		return
	}
	newSize := offsetsz * 2
	log.Println("Growing offset to:", newSize)
	newOff := make([]int64, newSize, newSize)
	copy(newOff, offset)
	offset = newOff
	offsetsz = newSize
	mutex.Unlock()
}
// 以下都是输入参数使用的，zero_copy.exe [options] 如果不想输入一些默认的ip与端口就直接输入一个文件名
func parseArgs() (host, port, mimetype string, procs int) {
	flag.Usage = Usage
	hostf := flag.String("h", P_DEFAULT_HOST, "Host or IP to listen on")
	portf := flag.String("p", P_DEFAULT_PORT, "Port to listen on")
	mimetypef := flag.String("m", P_DEFAULT_MIME_TYPE, "Mime type of file")
	procsf := flag.Int("c", 1, "Concurrent CPU cores to use.")
	flag.Parse()
	return *hostf, *portf, *mimetypef, *procsf
}
func Usage() {
	_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	_, _ = fmt.Fprintf(os.Stderr, "  %s [options] \n", os.Args[0])
	_, _ = fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
}