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
	ixs := make([]byte, len(enc))
	for i := range ixs { //copy indexes
		ixs[i] = enc[i][0]
	}

	cauchy := make([][]byte, m.n) //create matrix
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

	data := solve_from_inverse(m.inv, enc)
	fmt.Println(data)

	return nil, nil
}


func main() {
	m := NewManager(5, 3)
	data := []byte{18, 16, 12}
	enc, _ := m.Encode(data)
	/*
	for r := range(m.mat){
		fmt.Println(m.mat[r])		
	}
	*/
	fmt.Println("gott:", enc)
	fmt.Println("need: [149 238 12 219 68 106 151 182]")
	return
	

	zares := [][]byte{{2, enc[2]},{5, enc[5]},{7,enc[7]}}
	m.Decode(zares)
}
