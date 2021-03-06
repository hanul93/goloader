package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/hanul93/goloader"
	"github.com/hanul93/goloader/goobj"
	"github.com/kr/pretty"
)

type arrayFlags struct {
	File    []string
	PkgPath []string
}

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	s := strings.Split(value, ":")
	i.File = append(i.File, s[0])
	var path string
	if len(s) > 1 {
		path = s[1]
	}
	i.PkgPath = append(i.PkgPath, path)
	return nil
}

func main() {
	var files arrayFlags
	flag.Var(&files, "o", "load go object file")
	var pkgpath = flag.String("p", "", "package path")
	var parseFile = flag.String("parse", "", "parse go object file")
	var run = flag.String("run", "main.main", "run function")
	var times = flag.Int("times", 1, "run count")

	flag.Parse()

	if *parseFile != "" {
		parse(parseFile, pkgpath)
		return
	}

	if len(files.File) == 0 {
		flag.PrintDefaults()
		return
	}

	symPtr := make(map[string]uintptr)
	goloader.RegSymbol(symPtr)

	goloader.RegTypes(symPtr, time.Duration(0), time.Unix(0, 0))
	goloader.RegTypes(symPtr, runtime.LockOSThread)
	// most of time you don't need to register function, but if loader complain about it, you have to.
	goloader.RegTypes(symPtr, http.ListenAndServe, http.Dir("/"),
		http.Handler(http.FileServer(http.Dir("/"))), http.FileServer, http.HandleFunc,
		&http.Request{}, &http.Server{})
	w := sync.WaitGroup{}
	rw := sync.RWMutex{}
	goloader.RegTypes(symPtr, &w, w.Wait, &rw)
	symPtr["os.Stdout"] = *(*uintptr)(unsafe.Pointer(&os.Stdout))

	reloc, err := goloader.ReadObjs(files.File, files.PkgPath)
	if err != nil {
		fmt.Println(err)
		return
	}

	var mmapByte []byte
	for i := 0; i < *times; i++ {
		codeModule, err := goloader.Load(reloc, symPtr)
		if err != nil {
			fmt.Println("Load error:", err)
			return
		}
		runFuncPtr := codeModule.Syms[*run]
		if runFuncPtr == 0 {
			fmt.Println("Load error! not find function:", *run)
			return
		}
		funcPtrContainer := (uintptr)(unsafe.Pointer(&runFuncPtr))
		runFunc := *(*func())(unsafe.Pointer(&funcPtrContainer))
		runFunc()
		os.Stdout.Sync()
		codeModule.Unload()

		// a strict test, try to make mmap random
		if mmapByte == nil {
			mmapByte, err = goloader.Mmap(1024)
			if err != nil {
				fmt.Println(err)
			}
			b := make([]byte, 1024)
			copy(mmapByte, b) // reset all bytes
		} else {
			goloader.Munmap(mmapByte)
			mmapByte = nil
		}
	}

}

func parse(file, pkgpath *string) {

	if *file == "" {
		flag.PrintDefaults()
		return
	}

	// f, err := os.Open(*file)
	data, err := ioutil.ReadFile(*file)
	if err != nil {
		fmt.Printf("%# v\n", err)
		return
	}

	f := bytes.NewReader(data)

	obj, err := goobj.Parse(f, *pkgpath)
	pretty.Printf("%# v\n", obj)

	/*
	f.Close()
	if err != nil {
		fmt.Printf("error reading %s: %v\n", *file, err)
		return
	}
	*/
}
