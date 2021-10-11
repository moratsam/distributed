package main 
import (
	"fmt"

)

var prime = 0x11d
var exp_table = make([]byte, 512)
var log_table = make([]byte, 256)

func add(a, b byte) byte {
	return a^b
}

func sub(a, b byte) byte {
	return a^b
}

// calculate bit length
func length(a int) int {
	result := 0
	for i:=0; a>>i >0; i++ {
		result++
	}
	return result
}

func mult_costly(a, b byte) byte {
	result := 0
	for i:=0; a>>i > 0; i++ { //iterate over the bits of a
		if a & (1<<i) > 0 { // if current bit is 1
			result ^= int(b)<<i //xor b multiplied by this power of 2 to result
		}
	}

	len1, len2 := length(result), length(prime)
	if len1 < len2 {
		return byte(result)
	}
	for i:=len1-len2; i > -1; i-- { //while result is not smaller than the prime
		if result & (1<<(i+len2-1)) > 0{ //if current bit is 1
			result ^= prime << i //align divisor with the result and subtract its value
		}
	}
	return byte(result)
}


//use generator 2 to init log and exp tables
func init_tables() {
	x := byte(1)
	for i:=0; i<255; i++ {
		exp_table[i] = x
		log_table[x] = byte(i)
		x = mult_costly(x, 2)
	}
	for i:=255; i<512; i++ {
		exp_table[i] = exp_table[i-255]
	}
}

func mult(a, b byte) byte {
	if a==0 || b==0 {
		return 0
	}
	return exp_table[int(log_table[a]) + int(log_table[b])]
}

func div(a, b byte) byte {
	if a == 0{
		return 0
	} else if b == 0{
		panic("division by zero")
	} else{
		return exp_table[int(log_table[a]) + 255 - int(log_table[b])]
	}
}

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
	inv_mat [][]byte
	encoded []byte
}

func NewManager(k, n byte) *Manager {
	init_tables()
	mat := make([][]byte, n+k)
	for i := range mat {
		mat[i] = make([]byte, n)
	}

	m := &Manager{
		k:		k,
		n:		n,
		mat: mat,
	}
	m.create_cauchy()

	return m
}


func main() {

	m := NewManager(5, 3)
	for i := range m.mat {
		fmt.Println(m.mat[i])
	}
}
