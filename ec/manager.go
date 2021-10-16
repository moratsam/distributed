package main
import (
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
)

//------------------------------------

//create cauchy matrix of dimensions (n+k)xn
//every n rows suffice to reconstruct the data
func (m *Manager) create_cauchy() {
	k, n := m.k, m.n

	var i, j byte
	for i=0; i<n+k; i++ {
		for j=n+k; j<2*n+k; j++ {
			m.mat[i][j-n-k] = div(1, add(i, j))
		}
	}
}

type Manager struct {
	k, n byte
	mat [][]byte
	inv [][]byte
	enc []byte
}

func NewManager(k, n byte) *Manager {
	if int(k) + int(2*n) > 255 {
		panic("the sum of k and n must not exceed 255")
	}
	mat := create_cauchy(k, n)

	m := &Manager{
		k:		k,
		n:		n,
		mat: mat,
		inv: nil,
	}

	return m
}

func (m *Manager) Encode(data [][]byte) ([][]byte, error) {

	if len(data) != int(m.n) {
		return nil, errors.New("incorrect data length")
	}
	
	return encode(data, m.mat), nil
}

//enc is [[ix1, enc1], [ix2, enc2]..], where ix gives row of cauchy matrix
func (m *Manager) Decode(enc [][]byte) ([]byte, error) {
	if len(enc) < int(m.n) {
		return nil, errors.New("not enough encoded parts to reconstruct original data")
	}

	//TODO sort the indexes ASC
	//copy first n indexes of the encoded parts
	//needed to know which rows of the cauchy matrix were used to encrypt data
	ixs := make([]byte, m.n)
	for i := range ixs { 
		ixs[i] = enc[i][0]
	}

	cauchy := make([][]byte, m.n) //create matrix for cauchy
	for i := range cauchy {
		cauchy[i] = make([]byte, m.n)
	}

	for i := range cauchy { //populate it with rows from cauchy matrix
		for j := range cauchy {
			cauchy[i][j] = m.mat[ixs[i]][j]
		}
	}

	get_LU(cauchy)
	m.inv = invert_LU(cauchy)
	data := solve_from_inverse(m.inv, enc[0:m.n])

	control := "kurba sem DOBR  BOBR"
	ix := 0
	for _,data_part := range data{
		for _, c := range(data_part){
			if c != control[ix] {
				return nil, errors.New("neki ne dela")
			}
			ix++
		}
	}
	return nil, nil
}



var chunk_size = 1024

type chunk struct {
	size int
	data []byte
}


//	receive a chunk of bytes of data; 
//	for each row r in cauchy matrix:
//		make e := dot product <r, d>
//		send e to writer of r
func eencode(c_reader chan chunk, c_writers []chan byte){
	k, n := 5, 3
	//after main routine receives new d, it will use the c_data_available to tell each of the
	//row routines that a new d is available
	c_data_available := make([]chan struct{}, len(c_writers))

	//array of n bytes of data
	var data_chunk chunk
	wg := new(sync.WaitGroup)

	for i := range c_data_available{ //for every matrix row
		c_data_available[i] = make(chan struct{})

		go func(c_data_available chan struct{}, c_writer chan byte, data *chunk, wg *sync.WaitGroup) { 
			for {
				_, ok := <- c_data_available //new data is available
				if !ok{ //no more data
					close(c_writer)
					return
				}
				//enc := encode(data) //encode row with new n-chunk
				for _,b := range data.data[:data.size] {
					c_writer <- b
				}
				//c_writer <- data.data[:data.size] //send encoded byte to writer
				wg.Done() //signify no longer need current_chunk
			}
		}(c_data_available[i], c_writers[i], &data_chunk, wg)
	}

	ok := true
	for {
		data_chunk, ok = <- c_reader //receive chunk
		if !ok{ //channel closed
			for i := range c_data_available{
				close(c_data_available[i])
			}
			return
		}
		fmt.Println(data_chunk.size)
		wg.Add(n+k) //set up wait for routines
		for i := range c_data_available { //tell routines they may read chunk
			c_data_available[i] <- struct{}{}
		}
		wg.Wait() //wait for them to tell you they are finished
	}
}


func main() {
	k, n := 5, 3
	filepath := "fajl"

	c_reader := make(chan chunk)
	go readFile(filepath, c_reader) //make routine which will read file

	c_writers := make([]chan byte, n+k)//one chan for each writer which will write encoded file
	for i := range c_writers {
		c_writers[i] = make(chan byte, 3)
		outpath := filepath + "_" + strconv.Itoa(i) + ".enc"
		go writeFile(outpath, c_writers[i]) //spawn n+k routines which will write encoded files
	}

	go eencode(c_reader, c_writers) //read data, encode it, send it to writers
	time.Sleep(1*time.MiliSecond)
}

func writeFile(path string, c chan byte) {
  file, err := os.Create(path)
  if err != nil {
		fmt.Println(err)
  }
  defer file.Close()

	buf := make([]byte, chunk_size)
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
func readFile(path string, c chan chunk) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		fmt.Println("napakica v branju fajla frent")
	}

	for {
		var chunky chunk
		chunky.data = make([]byte, chunk_size)

		chunky.size, err = file.Read(chunky.data)
		if err != nil {
			if err == io.EOF{
				if chunky.size > 0{
					c <- chunky
				}
				close(c)
				break
			} else{
				fmt.Println(err)
			}
		}
		c <- chunky
	}
}

/*
func main() {

	readFile("fajl")
	return

	m := NewManager(5, 4)
	data := [][]byte{{'k', 'u', 'r', 'b', 'a'}, {' ', 's', 'e', 'm', ' '}, {'D', 'O', 'B', 'R', ' '}, {' ', 'B', 'O', 'B', 'R'}}
	enc, _ := m.Encode(data)

	//make n-subset of [enc], which will be put to Decode to retrieve original [data]
	for i:=0; i<len(enc); i++{
		for j:=1+i; j<len(enc); j++ {
			for k:=1+j; k<len(enc); k++{
				for l:=1+k; l<len(enc); l++{
					subset := [][]byte{enc[i], enc[j], enc[k], enc[l]}
					_, err := m.Decode(subset)
					if err != nil{
						fmt.Println(err)
					}
				}
			}
		}
	}

	//zares := [][]byte{{2, enc[2]},{5, enc[5]},{7,enc[7]}}
	//m.Decode(zares)
}
*/
