package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
	"syscall"
	"text/template"
)

const (
	DEFAULT_HOST      = "127.0.0.1"
	DEFAULT_PORT      = "8080"
	DEFAULT_MIME_TYPE = "text/plain"
	P_HEAD_TMPL         = "HTTP/1.0 200 OK\r\nCache-Control: max-age=31536000\r\nExpires: Thu, 31 Dec 2037 23:55:55 GMT\r\nContent-Type: {{.Mime}}\r\nContent-Length: {{.Length}}\r\n\r\n"

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

func main() {
	var oerr error
	host, port, mimetype, procs, filename := parseArgs()
	fmt.Printf("服务器文件名为： %s 描述消息内容类型为： %s\n", filename, mimetype)
	fmt.Printf("监听的地址：端口 %s:%s\n", host, port)
	// 设置使用多少核，值得注意的是golang默认使用的是单核处理
	runtime.GOMAXPROCS(procs)
	log.Println("设置的核心数为： ", procs)
	src, oerr = os.Open(filename)
	if oerr != nil {
		log.Fatal("Error opening payload. ", oerr)
	}
	fileinfo, serr := src.Stat()
	if serr != nil {
		log.Fatal("Error Stat on payload")
	}
	size = fileinfo.Size()
	srcfd = int(src.Fd())
	log.Println("文件大小为：", size)
	log.Println("文件描述符为：", srcfd)
	tmpl, terr := template.New("headers").Parse(HEAD_TMPL)
	if terr != nil {
		log.Fatal("Error parsing HEAD_TMPL", terr)
	}
	tmplData := struct {
		Mime   string
		Length int64
	}{mimetype, size}
	headBuf := &bytes.Buffer{}
	terr = tmpl.Execute(headBuf, tmplData)
	if terr != nil {
		log.Fatal("Error executing header template", terr)
	}
	headers = headBuf.String()
	// 预热一下操作系统的页缓存
	_, _ = ioutil.ReadAll(src)
	_, _ = src.Seek(0, os.SEEK_SET)

	addr := host + ":" + port
	sock, lerr := net.Listen("tcp", addr)
	if lerr != nil {
		log.Fatal("Error listening on ", addr, ". ", lerr)
	}
	for {
		conn, aerr := sock.Accept()
		if aerr != nil {
			log.Fatal("Error Accept. ", aerr)
		}
		go handle(conn)
	}
}
func handle(conn net.Conn) {
	log.Println("handle")
	var rerr, werr error
	var wrote int
	buf := make([]byte, 32*1024)
	wrote, werr = conn.Write([]byte(headers))
	if werr != nil {
		log.Fatal("Error writing headers", werr)
	}
	if wrote != len([]byte(headers)) {
		log.Fatal("Error: Wrote ", wrote, " headers bytes. Expected ", len([]byte(headers)))
	}
	outfile, ferr := conn.(*net.TCPConn).File()
	if ferr != nil {
		log.Fatal("Error getting conn fd", ferr)
	}
	outfd := int(outfile.Fd())
	if outfd >= offsetsz {
		growOffset(outfd)
	}
	currOffset := &offset[outfd]
	for *currOffset < size {
		// 零拷贝我是接收到了一个连接，可是我采用syscall.Sendfile()，我怎么转发这个端口？？？？
		// 思考1：我如果read出来，那么就会把数据读取了，那这样 网卡=》应用缓冲区
		// sendfile有四个参数：outfd int, infd int, offset *int64, count int
		//outfd是带读出内容的文件描述符、infd是待写入的内容的文件描述符、
		//offset是指定从文件流的哪个位置开始读（为空默认从头开始读）、count参数指定文件描述符in_fd和out_fd之间传输的字节数
		// in_fd必须是一个支持类似mmap函数的文件描述符（也就是必须指向真实文件）、out_fd是一个socket
		wrote, werr = syscall.Sendfile(outfd, srcfd, currOffset, int(size))
		if werr != nil {
			log.Fatal("Sendfile error:", werr)
		}
	}
	offset[outfd] = 0
	werr = conn.(*net.TCPConn).CloseWrite()
	if werr != nil {
		log.Println("Error on CloseWrite", werr)
	}
	var _ map[string]string
	// Consume input
	for {
		_, rerr = conn.Read(buf)
		if rerr == io.EOF {
			break
		} else if rerr != nil {
			log.Println("Error consuming read input: ", rerr)
			break
		}
	}
	werr = outfile.Close()
	if werr != nil {
		log.Println("Error on outfile Close", werr)
	}
	werr = conn.Close()
	if werr != nil {
		log.Println("Error on Close", werr)
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

// 以下都是输入参数使用的，zero_copy.exe [options] [filename] 如果不想输入一些默认的ip与端口就直接输入一个文件名
func parseArgs() (host, port, mimetype string, procs int, filename string) {
	flag.Usage = Usage
	hostf := flag.String("h", DEFAULT_HOST, "Host or IP to listen on")
	portf := flag.String("p", DEFAULT_PORT, "Port to listen on")
	mimetypef := flag.String("m", DEFAULT_MIME_TYPE, "Mime type of file")
	procsf := flag.Int("c", 1, "Concurrent CPU cores to use.")
	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(1)
	}
	flag.Parse()
	return *hostf, *portf, *mimetypef, *procsf, flag.Arg(0)
}
func Usage() {
	_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	_, _ = fmt.Fprintf(os.Stderr, "  %s [options] [filename]\n", os.Args[0])
	_, _ = fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
}
