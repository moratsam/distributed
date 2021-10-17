package main
import (
	"fmt"
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
	for i := range data[0]{
		for j := range data {
			fmt.Printf("%c", data[j][i])
		}
	}

	/*
	for _,data_part := range data{
		for _, c := range(data_part){
			fmt.Printf("%c", c)
		}
	}
	*/
	fmt.Println()
	return nil, nil
}




type chunk struct {
	size int
	data []byte
}


//	receive a chunk of bytes of data; 
//	for each row r in cauchy matrix:
//		make e := dot product <r, d>
//		send e to writer of r
func (m *Manager) eencode(c_reader chan chunk, c_writers []chan byte){
	k, n := 5, 3
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
						if i == 1111 && z == 0 {
							fmt.Println("z", z, " ix", ix)
							fmt.Println("\tbyte: ", encoded_byte)
							fmt.Println("\tcau: ", cauchy_row[ix])
							fmt.Println("\tdat: ", data.data[z+ix])
							fmt.Println("\tmul: ", mul(cauchy_row[ix], data.data[z+ix]))
							fmt.Println("\tadd: ", add(encoded_byte, mul(cauchy_row[ix], data.data[z+ix])))
							fmt.Println()
						}
						encoded_byte = add(encoded_byte, mul(cauchy_row[ix], data.data[z+ix]))
					}
					/*
					if i == 0{
						fmt.Println(i, cauchy_row)
						fmt.Println(i, data.data[z:z+int(m.n)])
						fmt.Println(i, encoded_byte)
					}
					*/
					c_writer <- encoded_byte //send it to writer
				}

				//c_writer <- data.data[:data.size] //send encoded byte to writer
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


func (m *Manager) ddecode(){
	cauchy := make([][]byte, m.n) //create matrix for cauchy
	subset := []int{3, 1, 0}
	filepaths := make([]string, m.n)
	for i,el := range subset {
		filepaths[i] = "fajl_" + strconv.Itoa(el) + ".enc"
	}

	c_row_indexes := make(chan int)
	c_encoded_data := make(chan chunk)
	go readEncoded(filepaths, c_row_indexes, c_encoded_data, m.chunk_size)

	i := 0
	for row_index := range c_row_indexes {
		subset[i] = row_index
		i++
	}

	for i := range cauchy {
		cauchy[i] = make([]byte, m.n)
	}

	for i := range cauchy { //populate it with rows from cauchy matrix
		for j := range cauchy {
			cauchy[i][j] = m.mat[subset[i]][j]
		}
	}

	get_LU(cauchy)
	m.inv = invert_LU(cauchy)
		
	c_writer := make(chan byte, 2*int(m.n))
	defer close(c_writer)
	outpath := "dekodiran"
	go writeFile(outpath, c_writer, m.chunk_size) 



	for chunky := range c_encoded_data{
		fmt.Println("chunk size: ", chunky.size)
		fmt.Println("chunk data: ", chunky.data[:chunky.size])
		for ix:=0; ix<chunky.size; ix+=int(m.n) {
			//decode
			data_word := decode_word(m.inv, chunky.data[ix:ix+int(m.n)])
			//send to writer
			for _, b := range data_word {
				fmt.Printf("%c", b)
				c_writer <- b
			}
		}
	}
}


func nov_main() {
	m := NewManager(5, 3)



	filepath := "fajl"

	c_reader := make(chan chunk)
	go readFile(filepath, c_reader, m.chunk_size) //make routine which will read file

	c_writers := make([]chan byte, m.n+m.k)//one chan for each writer which will write encoded file
	for i := range c_writers {
		c_writers[i] = make(chan byte, 3)
		outpath := filepath + "_" + strconv.Itoa(i) + ".enc"
		go writeFile(outpath, c_writers[i], m.chunk_size) //spawn n+k routines which will write encoded files
	}

	go m.eencode(c_reader, c_writers) //read data, encode it, send it to writers
	time.Sleep(1*time.Second)
	m.ddecode()
}


func main(){
	nov_main()
	fmt.Println("\n\n")
	//cain()
	//time.Sleep(1*time.Second)
}

/*
*/
//encoded kajla: [[0 164 172] [1 9 89] [2 60 173] [3 155 14] [4 23 199] [5 163 103] [6 29 156] [7 120 181]]
//encoded oo: [[0 222] [1 70] [2 74] [3 183] [4 95] [5 194] [6 58] [7 197]]

func cain() {

	m := NewManager(5, 3)
	data := [][]byte{{111}, {111}, {10}}
	//[[0 222] [1 70] [2 74] [3 183] [4 95] [5 194] [6 58] [7 197]]
	fmt.Println("pravilno")
	fmt.Println(m.mat[0])
	enc, _ := m.Encode(data)
	fmt.Println(enc)

	subset := [][]byte{enc[0], enc[1], enc[3]}
	m.Decode(subset)
	return

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
