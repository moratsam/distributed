package main
import (
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
	for {
		enc_byte := make([]byte, 1)
		for _, k := range keys { //read one byte before moving to the next shard
			file := file_handles[row_to_shardpath[k]]
			_, err := file.Read(enc_byte)
			if err != nil {
				if err == io.EOF {
					files_closed = true
					//break can be used here because all shards are the same size, which means
					//they all close at the same time (after same amount of bytes read).
					break
				} else {
					fmt.Println(err)
					return
				}
			}
			chunk.data[chunk.size] = enc_byte[0]
			chunk.size++
		}
		
		//it is save to consider chunk fulness only after every n bytes have been read
		//because const_CHUNK_SIZE is a multiple of n.
		if chunk.size == const_CHUNK_SIZE { 
			c_encoded_data <- chunk
			chunk.size = 0
		}
		if files_closed{
			break
		}
	}

	if chunk.size > 0 {
		c_encoded_data <- chunk //send the last chunk, even if it's not full
	}
}

func writeFile(path string, c chan byte, c_done chan struct{}) {
	file, err := os.Create(path)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()
	defer close(c_done)

	buf := make([]byte, const_CHUNK_SIZE)
	ix := 0
	for b := range c { //receive byte, write to file
		buf[ix] = b
		ix++
		if ix == len(buf)-1 {
			ix = 0
			_, err := file.Write(buf)
			if err != nil {
				fmt.Println(err)
			}
		}
	}

	if ix > 0 {
		_, err := file.Write(buf[:ix])
		if err != nil {
			fmt.Println(err)
		}
	}

}

//read bytes from file, send them to channel c
func readFile(path string, c chan data_chunk) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		errors.New("napakica v branju fajla frent")
	}

	for {
		var chunk data_chunk
		chunk.data = make([]byte, const_CHUNK_SIZE)

		chunk.size, err = file.Read(chunk.data)
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
}

