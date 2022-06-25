package main
import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	_"github.com/pkg/errors"
)

//------------------------------------

type Manager struct {
	k, n byte
	mat [][]byte
	enc []byte
}

func NewManager(k, n byte) *Manager {
	const_CHUNK_SIZE = 121*int(n)
	if int(k) + int(2*n) > 255 {
		panic("the sum of k and n must not exceed 255")
	}

	mat := create_cauchy(k, n)
	m := &Manager{
		k:		k,
		n:		n,
		mat: mat,
	}
	return m
}

//the function will receive chunks of data via c_reader, one at a time.
//it spawns a subroutine for each for in the cauchy matrix
//each subroutine will read the current chunk, break it into n-words, encode the n-words by
//making a dot product with its cauchy row, then send the encoded byte to its writer routine.
func (m *Manager) encode(c_reader chan data_chunk, c_writers []chan byte, fSize uint64){
	k, n := int(m.k), int(m.n)
	//each time the routine running encode() receives a new data chunk, it will use the
	//c_data_available to tell each of the row routines that a new chunk is available.
	c_data_available := make([]chan struct{}, len(c_writers))

	var chunk data_chunk //this chunk will be read by every row subroutine
	wg := new(sync.WaitGroup)

	for i := range c_data_available{ //for every matrix row
		c_data_available[i] = make(chan struct{})

		go func(c_data_available chan struct{}, c_writer chan byte, data *data_chunk, wg *sync.WaitGroup, i int) { 
			c_writer <- byte(i) //send index of cauchy matrix row to be stored in the shard
			cauchy_row := m.mat[i]

			bs := make([]byte, 8)
			binary.LittleEndian.PutUint64(bs, fSize)
			for _,b := range bs {
				c_writer <- b
			}

			for {
				_, ok := <- c_data_available //new data is available
				if !ok{ //no more data
					close(c_writer)
					return
				}
				for z:=0; z<data.size; z+=int(m.n){ //for every n-word of data
					var encoded_byte byte
					for ix := range cauchy_row { //do dot product of the n-word with cauchy row
						encoded_byte = add(encoded_byte, mul(cauchy_row[ix], data.data[z+ix]))
					}
					c_writer <- encoded_byte //send it to writer
				}

				wg.Done() //signify data chunk can be overwritten
			}
		}(c_data_available[i], c_writers[i], &chunk, wg, i)
	}

	totalRead := 0
	ok := true
	for {
		chunk, ok = <- c_reader //receive chunk
		if !ok{ //channel closed
			for _,c := range c_data_available{
				close(c)
			}
			break
		}
		//if the filesize of the file being encoded is not a multiple of n,
		//the last word of the last chunk would not be a n-word,
		//which would fuck up matrix operations. To prevent this, add zeros if necessary.
		if chunk.size % int(m.n) != 0 { //add up some zeros
			fmt.Println("neki caram tuki")
			chunk.size += (chunk.size % int(m.n))
		}
		wg.Add(n+k) //set up waitgroup to wait until subroutines finish processing chunk
		for _,c := range c_data_available { //tell routines they may read chunk
			c <- struct{}{}
		}
		wg.Wait() //wait for them to finish
		totalRead += chunk.size
	}
	fmt.Println("total read", totalRead)
}

//readShards will read the shards, and send back their indexes, so that the appropriate
//cauchy submatrix may be created. Then it will read shards in asc order and send encoded 
//data chunks via c_encoded_data.
//an inverse matrix will be created from the cauchy submatrix and will be used to decode the
//data, one n-word at a time. Finally, this will be passed to the writeFile routine, which
//will write the decoded data to a file.
func (m *Manager) Decode(shard_paths []string, outpath string){

	c_row_indexes := make(chan int) //c via which readShards() sends shard (row) indexes
	c_encoded_data := make(chan data_chunk) //c via which readShards() sends shard data
	c_writer := make(chan byte, 1*int(m.n)) //c via which decoded data is sent to writeFile()
	c_writer_done := make(chan struct{}) //c used by writeFile() to signal when it is done

	go readShards(shard_paths, c_row_indexes, c_encoded_data)
	go writeFile(outpath, c_writer, c_writer_done) 

	padding := <- c_row_indexes
	paddingBuf := make([]byte, 0, padding)
	fmt.Println("read padding", padding)
	i, row_indexes := 0, make([]int, m.n) //indexes of cauchy rows that encoded the files
	for row_index := range c_row_indexes {
		row_indexes[i] = row_index
		i++
	}

	inv := create_inverse(m.mat, row_indexes)

	oo := make([]byte, 0)
	doit := true
	for chunky := range c_encoded_data{
		for z := 0; z < chunky.size; z++{
			//fmt.Println("ooo")
			oo = append(oo, chunky.data[z])
		}
		if len(oo) >= 300 && doit {
			//fmt.Println("z:", oo)
			doit = false
		}
		for ix:=0; ix<chunky.size; ix+=int(m.n) {
			enc_word := chunky.data[ix:ix+int(m.n)]
			data_word := decode_word(inv, enc_word) //decoded
			//fmt.Println("enc word", enc_word, "dec word", data_word)
			for _, b := range data_word { //send to writer
				if len(paddingBuf) != cap(paddingBuf) {
					paddingBuf = append(paddingBuf, b)
					continue
				}
				c_writer <- b
			}
		}
	}
	//fmt.Println("xxxxx", oo)
	close(c_writer)
	for _ = range c_writer_done {} //wait for chan to close
}

//encodes file given by inpath, returns paths to shards (encoded files)
func (m *Manager) Encode(inpath string) ([]string, error) {
	outpaths := make([]string, m.n+m.k) //paths to shards
	c_reader := make(chan data_chunk) //c via which readFile() will send data
	c_writers := make([]chan byte, m.n+m.k)//c for each fileWrite() routine which will write shard
	c_writers_done := make([]chan struct{}, m.n+m.k) //c for each fileWrite() to signal when it is done

	for i := range c_writers {
		c_writers[i] = make(chan byte, int(m.n))
		outpaths[i] = inpath + "_" + strconv.Itoa(i) + ".enc"
		c_writers_done[i] = make(chan struct{})
		go writeFile(outpaths[i], c_writers[i], c_writers_done[i]) //spawn n+k routines which will write shards
	}

	fi, err := os.Stat(inpath);
	if err != nil {
		fmt.Println("Can't get file size")
	}
	fSize := int64(fi.Size())
	padding := uint64(int64(m.n) - (fSize % int64(m.n)))

	go readFile(inpath, c_reader, m.n) //make routine which will read file
	go m.encode(c_reader, c_writers, padding) //read data, encode it, send it to writers

	//wait for every writer to be done
	for _, c := range c_writers_done{
		for _ = range c {}
	}

	return outpaths, nil
}


func main(){
	m := NewManager(3, 7)
	outpaths, _ := m.Encode("fajl")
	

	//create random subset of out files to use for decoding
	//rand.Seed(time.Now().UnixNano())
	_ = time.Now()
	rand.Seed(3)
	subset := make([]string, m.n)
	for i, path_ix := range rand.Perm(int(m.k+m.n)){
		if i == int(m.n) {
			break
		}
		subset[i] = outpaths[path_ix]
	}
	//fmt.Println(subset)

	m.Decode(subset, "dekodiran")
}


func main1(){
	const_CHUNK_SIZE = 121
	inpath := "fajl.jpg"
	outpath := "out.jpg"
	c_reader := make(chan data_chunk) //c via which readFile() will send data
	c_writer := make(chan byte)
	c_writer_done := make(chan struct{})
	
	fi, err := os.Stat(inpath);
	if err != nil {
		fmt.Println("Can't get file size")
	}
	fSize := fi.Size()
	fmt.Println("fsize", fSize)

	go readFile(inpath, c_reader, 1)
	go writeFile(outpath, c_writer, c_writer_done)

	total := 0
	for chunk := range c_reader{
		for i:=0; i<chunk.size; i++ {
			c_writer <- chunk.data[i]
		}
		total += chunk.size
	}
	close(c_writer)

	 <- c_writer_done
	 fmt.Println("total: ", total)


}

