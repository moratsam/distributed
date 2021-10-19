package main
import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	_"github.com/pkg/errors"
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
	chunk_size int
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
		chunk_size: 1000*int(n),
		mat: mat,
		inv: nil,
	}

	return m
}

type chunk struct {
	size int
	data []byte
}


//	receive a chunk of bytes of data; 
//	for each row r in cauchy matrix:
//		make e := dot product <r, d>
//		send e to writer of r
func (m *Manager) encode(c_reader chan chunk, c_writers []chan byte){
	k, n := int(m.k), int(m.n)
	//after main routine receives new d, it will use the c_data_available to tell each of the
	//row routines that a new d is available
	c_data_available := make([]chan struct{}, len(c_writers))

	//array of n bytes of data
	var data_chunk chunk
	wg := new(sync.WaitGroup)

	for i := range c_data_available{ //for every matrix row
		c_data_available[i] = make(chan struct{})

		go func(c_data_available chan struct{}, c_writer chan byte, data *chunk, wg *sync.WaitGroup, i int) { 
			c_writer <- byte(i) //send index of cauchy matrix row
			cauchy_row := m.mat[i]

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

				wg.Done() //signify no longer need current_chunk
			}
		}(c_data_available[i], c_writers[i], &data_chunk, wg, i)
	}

	ok := true
	for {
		data_chunk, ok = <- c_reader //receive chunk
		if !ok{ //channel closed
			for _,c := range c_data_available{
				close(c)
			}
			return
		}
		wg.Add(n+k) //set up wait for routines

		if data_chunk.size % int(m.n) != 0 { //add up some zeros
			fmt.Println("neki caram tuki")
			data_chunk.size += (data_chunk.size % int(m.n))
		}


		for _,c := range c_data_available { //tell routines they may read chunk
			c <- struct{}{}
		}
		wg.Wait() //wait for them to tell you they are finished
	}
}

func (m *Manager) create_cauchy_submatrix(row_indexes []int) [][]byte {
	submat := make([][]byte, m.n) //create matrix for cauchy
	for i := range submat{
		submat[i] = make([]byte, m.n)
	}

	for i := range submat { //populate it with rows from whole cauchy matrix
		submat[i] = m.mat[row_indexes[i]][:]
	}
	return submat
}


func (m *Manager) Decode(enc_paths []string){

	c_row_indexes := make(chan int)
	c_encoded_data := make(chan chunk)
	go readEncoded(enc_paths, c_row_indexes, c_encoded_data, m.chunk_size)


	i, row_indexes := 0, make([]int, m.n) //indexes of cauchy rows that encoded the files
	for row_index := range c_row_indexes {
		row_indexes[i] = row_index
		i++
	}

	cauchy := m.create_cauchy_submatrix(row_indexes)

	get_LU(cauchy)
	m.inv = invert_LU(cauchy)
		
	c_writer := make(chan byte, 2*int(m.n))
	c_writer_done := make(chan struct{})
	outpath := "dekodiran"
	go writeFile(outpath, c_writer, m.chunk_size, c_writer_done) 

	for chunky := range c_encoded_data{
		//fmt.Println("chunk size: ", chunky.size)
		//fmt.Println("chunk data: ", chunky.data[:chunky.size])
		for ix:=0; ix<chunky.size; ix+=int(m.n) {
			//decode
			data_word := decode_word(m.inv, chunky.data[ix:ix+int(m.n)])
			//send to writer
			for _, b := range data_word {
				//fmt.Printf("%c", b)
				c_writer <- b
			}
		}
	}
	close(c_writer)
	for _ = range c_writer_done {} //just wait for chan to close
}

//encodes file given by in_path, returns paths to encoded files
func (m *Manager) Encode(in_path string) ([]string, error) {
	c_reader := make(chan chunk)
	go readFile(in_path, c_reader, m.chunk_size) //make routine which will read file

	out_paths := make([]string, m.n+m.k)
	c_writers := make([]chan byte, m.n+m.k)//one chan for each writer which will write encoded file
	c_writers_done := make([]chan struct{}, m.n+m.k) //chan for writer to signal it is done
	for i := range c_writers {
		c_writers[i] = make(chan byte, 3)
		out_paths[i] = in_path + "_" + strconv.Itoa(i) + ".enc"
		c_writers_done[i] = make(chan struct{})
		go writeFile(out_paths[i], c_writers[i], m.chunk_size, c_writers_done[i]) //spawn n+k routines which will write encoded files
	}

	go m.encode(c_reader, c_writers) //read data, encode it, send it to writers


	//wait for every writer to be done
	for _, c := range c_writers_done{
		for _ = range c {} 	}

	return out_paths, nil
}


func main(){
	m := NewManager(5, 3)
	out_paths, _ := m.Encode("fajl")
	
	//create random subset of out files to use for decoding
	rand.Seed(time.Now().UnixNano())
	subset := make([]string, m.n)
	for i, path_ix := range rand.Perm(int(m.k+m.n)){
		if i == int(m.n) {
			break
		}
		subset[i] = out_paths[path_ix]
	}
	fmt.Println(subset)

	m.Decode(subset)
}

