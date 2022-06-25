package main
import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/pkg/errors"
)

//------------------------------------
var const_CHUNK_SIZE int

type data_chunk struct {
	size int
	data []byte
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


//open all shards and read their first byte to get the index of the cauchy row that was used
//to encode them. Sort the indexes asc, then send them back via the c_row_indexes channel.
//Then sort the shards and read one byte from the first, one from the second.. until all data //is read. Send this data back in chunks via the c_encoded_data channel.
//(Imagine that the n sorted shards make up a matrix with n rows. The data from this matrix
//is then read column-wise, from left to right).
func readShards(shardpaths []string, c_row_indexes chan int, c_encoded_data chan data_chunk){
	defer close(c_encoded_data)
	n := len(shardpaths)
	var err error

	//0->3 means that the first byte of file shardpaths[3] was '0'
	//ie. that shardpaths[3] was encoded using the 0th row of the encoding cauchy matrix
	row_to_shardpath := make(map[int]int)
	file_handles := make([]*os.File, n)

	pizda := make([]byte, 0)
	for i := range file_handles{
		file_handles[i], err = os.Open(shardpaths[i]) //open file
		if err != nil {
			errors.New("napakica v branju fajla, frend")
		}
		defer file_handles[i].Close()

		row_index := make([]byte, 1)
		_, err := file_handles[i].Read(row_index) //read row index that encoded this file
		if err != nil {
			fmt.Println(err)
		}
		row_to_shardpath[int(row_index[0])] = i //store row index in map

		paddingBuf := make([]byte, 8)
		_, err = file_handles[i].Read(paddingBuf)
		if err != nil {
			fmt.Println(err)
		}
		padding := binary.LittleEndian.Uint64(paddingBuf)
		if i == 0 {
			c_row_indexes <- int(padding)
		}
		pizda = append(pizda, row_index[0])
	}

	//sort the indexes
	keys := make([]int, 0, n)
	for k := range row_to_shardpath { //make array of row indexes
		keys = append(keys, k)
	}
	sort.Ints(keys) //sort the row indexes in asc

	//send sorted indexes back, they will be used to construct the cauchy sub-matrix
	for _, k := range keys {
		c_row_indexes <- k
	}
	close(c_row_indexes)

	var chunk data_chunk
	chunk.data = make([]byte, const_CHUNK_SIZE)
	files_closed := false
	oo := make([]byte, 0)
	for {
		for _, k := range keys { //read one byte before moving to the next shard
			file := file_handles[row_to_shardpath[k]]
			readSize, err := file.Read(chunk.data[chunk.size:chunk.size+1])
			if err != nil {
				if err == io.EOF {
					files_closed = true
					//break can be used here because all shards are the same size, which means
					//they all close at the same time (after same amount of bytes read).
					//chunk.size += readSize
					if readSize != 0 {
						fmt.Println("pomoje tole ni kul")
					}
					//fmt.Println("zadnji", readSize)
					break
				} else {
					fmt.Println(err)
					return
				}
			}
			/*
			for z:=0; z<readSize; z++ {
				pizda = append(pizda, chunk.data[chunk.size+z])
			}
			*/
			chunk.size += readSize
			if len(oo) == 300 {
				fmt.Println("rrrr:", oo)
			}
		}
		
		//it is save to consider chunk fulness only after every n bytes have been read
		//because const_CHUNK_SIZE is a multiple of n.
		if chunk.size >= const_CHUNK_SIZE { 
			for z:=0; z<chunk.size; z++ {
				pizda = append(pizda, chunk.data[z])
			}
			/*
			var copyChunk data_chunk
			copyChunk.data = make([]byte, len(chunk.data))
			for q:=0; q<chunk.size; q++ {
				copyChunk.data[q] = chunk.data[q]
			}
			copyChunk.size = chunk.size
			*/
			copyChunk := data_chunk{
				size: chunk.size,
				data: make([]byte, chunk.size),
			}
			copy(copyChunk.data, chunk.data)
			c_encoded_data <- copyChunk
			chunk.size = 0
		}
		if files_closed{
			break
		}
	}

	if chunk.size > 0 {
		fmt.Println("posl zadnga")
		c_encoded_data <- chunk //send the last chunk, even if it's not full
	}

	//fmt.Println("s read", pizda)
}

func writeFile(path string, c chan byte, c_done chan struct{}) {
	totalWrite := 0
	file, err := os.Create(path)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()
	defer close(c_done)

	pizda := make([]byte, 0)
	buf := make([]byte, const_CHUNK_SIZE)
	//buf := make([]byte, 1)
	ix := 0
	for b := range c { //receive byte, write to file
		pizda = append(pizda, b)
		buf[ix] = b
		ix++

		if ix >= len(buf) {
			ix = 0
			totalWrite += len(buf)
			_, err := file.Write(buf)
			if err != nil {
				fmt.Println(err)
			}
		}
	}

	//fmt.Println("total pre inc:", totalWrite, ix)
	if ix > 0 {
		totalWrite += ix
		_, err := file.Write(buf[:ix])
		if err != nil {
			fmt.Println(err)
		}
	}

	//fmt.Println("total write", totalWrite)
	//fmt.Println("output", pizda)
}

//read bytes from file, send them to channel c
func readFile(path string, c chan data_chunk, n byte) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		errors.New("napakica v branju fajla frent")
	}

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		errors.New("napakica v statu frent")
	}
	fmt.Println("fsize", stat.Size())
	fSize := stat.Size()

	// Some padding is necessary.
	padding := int(int64(n) - (fSize % int64(n)))
	fmt.Println("padding neccessary", padding)
	if padding > 0 {
		var chunk data_chunk
		chunk.data = make([]byte, const_CHUNK_SIZE)

		chunk.size, err = file.Read(chunk.data[padding:])
		chunk.size += padding
		if err != nil {
			if err == io.EOF{
				if chunk.size > 0{
					c <- chunk
				}
				close(c)
				return
			} else{
				fmt.Println(err)
			}
		}
		c <- chunk
	}

	pizda := make([]byte, 0)
	for {
		var chunk data_chunk
		chunk.data = make([]byte, const_CHUNK_SIZE)

		chunk.size, err = file.Read(chunk.data)
		for x := 0; x < chunk.size; x++ {
			pizda = append(pizda, chunk.data[x])
		}
		if err != nil {
			if err == io.EOF{
				if chunk.size > 0{
					c <- chunk
				}
				close(c)
				break
			} else{
				fmt.Println(err)
			}
		}
		c <- chunk
	}
	//fmt.Println("input", pizda)
}

