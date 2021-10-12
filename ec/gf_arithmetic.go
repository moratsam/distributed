package main 
import (
	"fmt"
	"github.com/pkg/errors"
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

func mul_costly(a, b byte) byte {
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
		x = mul_costly(x, 2)
	}
	for i:=255; i<512; i++ {
		exp_table[i] = exp_table[i-255]
	}
}

func mul(a, b byte) byte {
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
	inv [][]byte
	encoded []byte
}

func NewManager(k, n byte) *Manager {
	if int(k) + int(n) > 255 {
		panic("the sum of k and n must not exceed 255")
	}
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

// e:: encoded, (n+k)x1,  d: data, (n)x1
//e = (m.mat)*d
func (m *Manager) encode(data []byte) ([]byte, error) {
	k, n := m.k, m.n
	if len(data) != int(n) {
		return nil, errors.New("incorrect data length")
	}
	
	encoded := make([]byte, n+k)
	var i, j byte
	for i=0; i<n+k; i++ {
		for j=0; j<n; j++ {
			encoded[i] = add(encoded[i], mul(m.mat[i][j], data[j]))
		}
	}
	return encoded, nil
}

func (m *Manager) get_LU() {
	dim := m.n

	mat := m.inv
	var i, row_ix, col_ix byte
	for i=0; i<dim; i++{
		if mat[i][i] == 0{
			continue
		}
		for row_ix=i+1; row_ix<dim; row_ix++{
			//derive factor to destroy first elemnt
			mat[row_ix][i] = div(mat[row_ix][i], mat[i][i])
			//subtract (row i's element * factor) from every other element in row
			for col_ix=i+1; col_ix<dim; col_ix++{
				mat[row_ix][col_ix] = sub(mat[row_ix][col_ix], mul(mat[i][col_ix],mat[row_ix][i]))
			}
		}
	}

}

func (m *Manager) invert_LU() {
	dim := int(m.n)
	mat := m.inv

	side := make([][]byte, m.n) //create side identity matrix
	for i := range side {
		side[i] = make([]byte, m.n)
		side[i][i] = 1
	}

	//invert U by adding an identity to its side. When U becomes identity, side is inverted U.
	//no operations on U actually need to be performed, just their effects on the side
	//matrix are being recorded.
	var i, j, k int
	for i=dim-1; i>=0; i-- { //for every row
		for j=dim-1; j>i; j-- { //for every column
			for k=dim-1; k>=j; k-- { //subtract row to get a 0 in U, reflect this change in side
				side[i][k] = sub(side[i][k], mul(mat[i][j], side[j][k]))
			}
		}
		if mat[i][i] == 0{
			continue
		} else {
			//divide mat[i][i] by itself to get a 1, reflect this change in whole line of side
			for j=dim-1; j>=0; j-- {
				side[i][j] = div(side[i][j], mat[i][i])
			}
		}
	}

	//get inverse of L
	for i=0; i<dim; i++ {
		for j=0; j<i; j++ {
			for k=0; k<=j; k++ {
				//since an in-place algo was used for LU decomposition,
				//diagonal values of LU were overwritten by U,
				//whereas L expects them to be equal to 1
				//in this case, no mul should be performed (to simulate multiplying by 1)
				if j == k { 
					side[i][k] = sub(side[i][k], mat[i][j])
				} else {
					side[i][k] = sub(side[i][k], mul(mat[i][j], side[j][k]))
				}
			}
		}
	}


	//inverse matrix is now the side matrix! because m.inv kinda became identity matrix
	//kinda, because no changes to m.inv were actually recorded
	m.inv = side

}


//(U^-1)(L^-1)[enc] = [data]
func (m *Manager) solve_from_inverse(enc []byte) []byte {
	dim := int(m.n)
	var i, j int

	//calculate W := (L^-1)[enc]
	w := make([]byte, dim)
	for i=0; i<dim; i++ {
		for j=0; j<=i; j++ {
			if i == j { //diagonal values were overwritten, but pretend they're still 1
				w[i] = add(w[i], enc[j])
			} else {
				w[i] = add(w[i], mul(m.inv[i][j], enc[j]))
			}
		}
	}

	//calculate [data] = (U^-1)W
	data := make([]byte, dim)
	for i=dim-1; i>=0; i-- {
		for j=dim-1; j>=i; j-- {
			data[i] = add(data[i], mul(m.inv[i][j], w[j]))
		}
	}
	return data
}

//enc is [[ix1, enc1], [ix2, enc2]..], where ix gives row of cauchy matrix
func (m *Manager) Decode(enc [][]byte) ([]byte, error) {
	ixs := make([]byte, len(enc))
	for i := range ixs { //copy indexes
		ixs[i] = enc[i][0]
	}

	inv := make([][]byte, m.n) //create inverse matrix
	for i := range inv {
		inv[i] = make([]byte, m.n)
	}

	for i := range inv { //populate it with rows from cauchy matrix
		for j := range inv {
			inv[i][j] = m.mat[ixs[i]][j]
		}
	}
	m.inv = inv




	m.get_LU()
	m.invert_LU()
	data := m.solve_from_inverse([]byte{enc[0][1], enc[1][1], enc[2][1]})
	fmt.Println(data)

	return nil, nil
}

func main() {
	m := NewManager(5, 3)
	data := []byte{16, 12, 183}
	enc, _ := m.encode(data)
	/*
	for r := range(m.mat){
		fmt.Println(m.mat[r])		
	}
	*/
	fmt.Println(enc)

	zares := [][]byte{{2, enc[2]},{5, enc[5]},{7,enc[7]}}
	m.Decode(zares)
}
