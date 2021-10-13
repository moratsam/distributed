package main
import (
	"fmt"
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
	fmt.Println(data)

	return nil, nil
}


func main() {
	m := NewManager(5, 3)
	data := [][]byte{{'r', 23}, {16, 16}, {12, 5}}
	enc, _ := m.Encode(data)
	/*
	for r := range(m.mat){
		fmt.Println(m.mat[r])		
	}
	*/
	fmt.Println("gott:", enc)
	fmt.Println("need: [149 238 12 219 68 106 151 182]")
	
	//make n-subset of [enc], which will be put to Decode to retrieve original [data]
	for i:=0; i<len(enc); i++{
		for j:=1+i; j<len(enc); j++ {
			for k:=1+j; k<len(enc); k++{
				subset := [][]byte{enc[i], enc[j], enc[k]}
				m.Decode(subset)
				return
			}
		}
	}

	//zares := [][]byte{{2, enc[2]},{5, enc[5]},{7,enc[7]}}
	//m.Decode(zares)
}
