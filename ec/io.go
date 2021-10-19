package main
import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/pkg/errors"
)

//------------------------------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


func readEncoded(filepaths []string, c_row_indexes chan int, c_encoded_data chan chunk, chunk_size int){
	defer close(c_encoded_data)
	n := len(filepaths)
	//for example 0->3 means that the first byte of file filepaths[3] was '0'
	//ie. that filepaths[3] was encoded using the 0th row of the encoding cauchy matrix
	row_to_filepath := make(map[int]int)
	file_handles := make([]*os.File, n)
	var err error
	for i := range file_handles{
		file_handles[i], err = os.Open(filepaths[i]) //open file
		if err != nil {
			errors.New("napakica v branju fajla, frend")
		}
		defer file_handles[i].Close()

		row_index := make([]byte, 1)
		_, err := file_handles[i].Read(row_index) //read row index that encoded this file
		if err != nil {
			fmt.Println(err)
		}
		row_to_filepath[int(row_index[0])] = i //store row index in map
	}

	keys := make([]int, 0, n)
	for k := range row_to_filepath { //make array of row indexes
		keys = append(keys, k)
	}

	sort.Ints(keys) //sort the row indexes in asc
	//send sorted indexes back, they will be used to construct the cauchy sub-matrix
	for _, k := range keys {
		c_row_indexes <- k
	}
	close(c_row_indexes)



	var chunky chunk
	chunky.data = make([]byte, chunk_size)

	files_closed := false
	for {
		enc_byte := make([]byte, 1)
		for _, k := range keys {
			file := file_handles[row_to_filepath[k]]
			_, err := file.Read(enc_byte)
			if err != nil {
				if err == io.EOF {
					files_closed = true
					break
				} else {
					fmt.Println(err)
					return
				}
			}
			chunky.data[chunky.size] = enc_byte[0]
			chunky.size++
		}
		
		if chunky.size == chunk_size {
			c_encoded_data <- chunky
			chunky.size = 0
		}

		if files_closed{
			break
		}
	}

	c_encoded_data <- chunky
	return


	var tmp_chunk chunk
	tmp_chunk.data = make([]byte, chunk_size)

	for _, k := range keys { //for each file
		file := file_handles[row_to_filepath[k]]
	//	fmt.Println("handle num: ", row_to_filepath[k])

		for { //read encoded data chunks
			var chunky chunk
			chunky.data = make([]byte, chunk_size)

			chunky.size, err = file.Read(chunky.data)
			if err != nil {
				if err == io.EOF{
					if chunky.size == chunk_size{
						fmt.Println("chunky poln")
						c_encoded_data <- chunky
					} else { //put it aside until a full chunk can be created
						space_left_in_tmp_chunk := chunk_size - tmp_chunk.size
						chunky_data_leftover := chunky.size
						data_to_be_used := min(space_left_in_tmp_chunk, chunky_data_leftover)

						//add chunky'sdata to tmp_chunk. Then two things are possible:
						//either tmp_chunk is full or chunky is empty (or both)
						for i:=0; i<data_to_be_used; i++{
							tmp_chunk.data[i + tmp_chunk.size] = chunky.data[i]
						}
						tmp_chunk.size += data_to_be_used
						space_left_in_tmp_chunk -= data_to_be_used
						chunky_data_leftover -= data_to_be_used

						if space_left_in_tmp_chunk == 0 {
							fmt.Println("tmp_chunk poln")
							c_encoded_data <- tmp_chunk
							//copy remaining data from chunky to tmp_chunk
							tmp_chunk.size = 0
						}
						if chunky_data_leftover > 0{
							tmp_chunk.data = chunky.data[data_to_be_used : chunky.size]
							tmp_chunk.size = chunky_data_leftover
						}
					}
					break
				} else {
					fmt.Println(err)
					return
				}
			}

			if chunky.size == chunk_size{
				fmt.Println("chunky poln")
				c_encoded_data <- chunky
			} else { //put it aside until a full chunk can be created
				space_left_in_tmp_chunk := chunk_size - tmp_chunk.size
				chunky_data_leftover := chunky.size
				data_to_be_used := min(space_left_in_tmp_chunk, chunky_data_leftover)

				//add chunky'sdata to tmp_chunk. Then two things are possible:
				//either tmp_chunk is full or chunky is empty (or both)
				for i:=0; i<data_to_be_used; i++{
					tmp_chunk.data[i + tmp_chunk.size] = chunky.data[i]
				}
				tmp_chunk.size += data_to_be_used
				space_left_in_tmp_chunk -= data_to_be_used
				chunky_data_leftover -= data_to_be_used

				if space_left_in_tmp_chunk == 0 {
					fmt.Println("tmp_chunk poln")
					c_encoded_data <- tmp_chunk
					//copy remaining data from chunky to tmp_chunk
					tmp_chunk.size = 0
				}
				if chunky_data_leftover > 0{
					tmp_chunk.data = chunky.data[data_to_be_used : chunky.size]
					tmp_chunk.size = chunky_data_leftover
				}
			}
		}
	}

	if tmp_chunk.size > 0 {
		if tmp_chunk.size % n != 0 {
			fmt.Println("error, not right amount of data")
		} else {
		//	fmt.Println("size:", tmp_chunk.size)
		//	fmt.Println("data:", tmp_chunk.data[0:tmp_chunk.size])

			c_encoded_data <- tmp_chunk
		}
	}
}



func writeFile(path string, c chan byte, chunk_size int, c_done chan struct{}) {
	file, err := os.Create(path)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()
	defer close(c_done)

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
func readFile(path string, c chan chunk, chunk_size int) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		errors.New("napakica v branju fajla frent")
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
		/*
		for i := range chunky.data[:chunky.size] {
			fmt.Println(chunky.data[i])
		}
		fmt.Println("\n\n")
		*/
		c <- chunky
	}
}

