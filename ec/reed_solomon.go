package main
import (
	"fmt"
)

//------------------------------------

//create cauchy matrix of dimensions (n+k)xn
//every n rows suffice to reconstruct the data
func create_cauchy(k, n byte) [][]byte{
	mat := make([][]byte, n+k)
	for i := range mat {
		mat[i] = make([]byte, n)
	}

	var i, j byte
	for i=0; i<n+k; i++ {
		for j=n+k; j<2*n+k; j++ {
			mat[i][j-n-k] = div(1, add(i, j))
		}
	}
	return mat
}

//[data] has dimensions nxY for arbitrary Y
//[enc] has dimensions (n+k)x(1+Y)
//the first element of each row is index of mat row that was used to encode it
//[enc] = (mat)[data]
func encode(data [][]byte, mat [][]byte) [][]byte {
	k, n, data_columns := len(mat)-len(mat[0]), len(mat[0]), len(data[0])
	
	enc := make([][]byte, n+k)
	for i := range enc{
		enc[i] = make([]byte, 1+len(data[0]))
	}

	var r, j, y int
	for r=0; r<n+k; r++ { //for every row in mat
		enc[r][0] = byte(r) //record row index, which is needed in decoding
		for y=0; y<data_columns; y++{ //for every column in data
			for j=0; j<n; j++ { //make sum of (row*column)
/*
				if r == 0 && y == 0 {
					fmt.Println("r: ", r, "j: ", j)
					fmt.Println("\tenc[r]", enc[r][1+y])
					fmt.Println("\tcau", mat[r][j])
					fmt.Println("\tdat", data[j][y])
					fmt.Println("\tmul", mul(mat[r][j], data[j][y]))
					fmt.Println("\tadd", add(enc[r][1+y], mul(mat[r][j], data[j][y])))
					
				}
*/
				enc[r][1+y] = add(enc[r][1+y], mul(mat[r][j], data[j][y]))
			}
		}
	}
	return enc
}


func get_LU(mat [][]byte) {
	dim := byte(len( mat[0] ))

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

func invert_LU(mat [][]byte) [][]byte {
	dim := len( mat[0] )

	side := make([][]byte, dim) //create side identity matrix
	for i := range side {
		side[i] = make([]byte, dim)
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
	return side
}


func decode_word(inv [][]byte, enc []byte) []byte{
	dim := len(inv[0])

	//calculate W := (L^-1)[enc]
	w := make([]byte, dim)
	for r:=0; r<dim; r++ { //for every row in inv
	//	fmt.Println("inv row: ", inv[r])
		for j:=0; j<=r; j++ {
//			fmt.Println("r: ", r, "j: ", j)
//			fmt.Println("\tw[j]", w[r])
//			fmt.Println("\tenc[j]", enc[j])
			if r == j { //diagonal values were overwritten in LU, but pretend they're still 1
				w[r] = add(w[r], enc[j])
//				fmt.Println("\tadd", add(w[r], enc[j]))
			} else {
				w[r] = add(w[r], mul(inv[r][j], enc[j]))
//				fmt.Println("\tmul", mul(inv[r][j], enc[j]))
//				fmt.Println("\tadd", add(w[r], mul(inv[r][j], enc[j])))
			}
		}
	}

	data_word := make([]byte, dim)
	for r:=dim-1; r>=0; r-- {
		for j:=dim-1; j>=r; j-- {
			data_word[r] = add(data_word[r], mul(inv[r][j], w[j]))
		}
	}
	fmt.Println()
	return data_word
}

//(mat)[data] = [enc]
//(mat^-1)(mat)[data] = (mat^-1)[enc] ==> (mat^-1)[enc] = [data]
//mat = LU
//mat^-1 = (U^-1)(L^-1)  
//(U^-1)(L^-1)[enc] = [data]
func solve_from_inverse(inv, enc [][]byte) [][]byte {
	dim, data_columns := len(inv[0]), len(enc[0]) -1 //-1 because first el is index of row
	var r, y, j int

	w := make([][]byte, dim)
	for i := range w{
		w[i] = make([]byte, data_columns)
	}
	//calculate W := (L^-1)[enc]
	for r=0; r<dim; r++ { //for every row in inv
//		fmt.Println("row: ", inv[r])
		for y=0; y<data_columns; y++{ //for every column in data
			for j=0; j<=r; j++ { //make sum of (row*column)
				if y == 0 {
//					fmt.Println("r: ", r, "j: ", j)
//					fmt.Println("\tw[j]", w[r][y])
//					fmt.Println("\tenc[j]", enc[j][1+y])
					
				}
				if r == j { //diagonal values were overwritten, but pretend they're still 1
					w[r][y] = add(w[r][y], enc[j][1+y])
//					fmt.Println("\tadd", add(w[r][y], enc[j][1+y]))
				} else {
					w[r][y] = add(w[r][y], mul(inv[r][j], enc[j][1+y]))
//					fmt.Println("\tmul", mul(inv[r][j], enc[j][1+y]))
//					fmt.Println("\tadd", add(w[r][y], mul(inv[r][j], enc[j][1+y])))
				}
			}
		}
	}

	data := make([][]byte, dim)
	for i := range data{
		data[i] = make([]byte, data_columns)
	}
	//calculate [data] = (U^-1)W
	for r=dim-1; r>=0; r-- {
		for y=0; y<data_columns; y++{
			for j=dim-1; j>=r; j-- {
				data[r][y] = add(data[r][y], mul(inv[r][j], w[j][y]))
			}
		}
	}
	return data
}
