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
	encoded []byte
}

func NewManager(k, n byte) *Manager {
	if int(k) + int(n) > 255 {
		panic("the sum of k and n must not exceed 255")
	}
	mat := make([][]byte, n+k)
	for i := range mat {
		mat[i] = make([]byte, n)
	}
	mat := create_cauchy()

	m := &Manager{
		k:		k,
		n:		n,
		mat: mat,
	}

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


//mat = LU
//mat*[data] = [enc]
//(mat)(mat^-1)[data] = (mat^-1)[enc] ==> [data] = (mat^-1)[enc]
//mat^-1 = (U^-1)(L^-1)
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
	data := []byte{18, 16, 12}
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
