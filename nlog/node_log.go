package nlog

import (
	"fmt"
	"io"
	"log"
	"os"
	"simple_go/monoxide/params"
	"strconv"
)

type PbftLog struct {
	Plog *log.Logger
}

func NewPbftLog(sid uint64) *PbftLog {
	pfx := fmt.Sprintf("S%d: ", sid)
	writer1 := os.Stdout

	dirpath := params.LogWrite_path
	err := os.MkdirAll(dirpath, os.ModePerm)
	if err != nil {
		log.Panic(err)
	}
	writer2, err := os.OpenFile(dirpath+"/S"+strconv.Itoa(int(sid))+".log", os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Panic(err)
	}
	pl := log.New(io.MultiWriter(writer1, writer2), pfx, log.Lshortfile|log.Ldate|log.Ltime)
	fmt.Println()

	return &PbftLog{
		Plog: pl,
	}
}
