package main
import (
	"fmt"
	"math/rand"
	_"strconv"
	_"sync"
	"time"

	_"github.com/pkg/errors"
	"go.uber.org/zap"
)

//------------------------------------

type Manager struct {
	logger *zap.Logger

	k, n byte
	secret byte
	poly []byte
}

func NewManager(k, n, secret byte) *Manager {
	if int(k) + int(2*n) > 255 {
		panic("the sum of k and n must not exceed 255")
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	m := &Manager{
		k:		k,
		n:		n,
		secret: secret,
		poly: []byte{secret, 8, 73},
		logger: logger,
	}
	return m
}

func (m *Manager) poly_eval(x byte) byte {
	value := byte(0)
	for ix, coef := range(m.poly){
		powered_x := byte(1)
		for z:=0; z<ix; z++{ //power up x
			powered_x = mul(powered_x, x)
		}
//		m.logger.Debug("", zap.Uint8("coef", coef), zap.Uint8("value here", mul(coef, powered_x)))
		value = add(value, mul(coef, powered_x))
//		m.logger.Debug("", zap.Uint8("coef", coef), zap.Uint8("total value here", value))
	}

//	m.logger.Debug("", zap.Uint8("x", x), zap.Uint8("poly evaluated at x", value))
//	m.logger.Debug("\n\n")
	return value
}


type point struct {
	x, y byte
}

func (m *Manager) interpolate(points []point) byte {
	lagrange_basis_coef := make([]byte, len(points))

	var ss byte
	for ix, _ := range(lagrange_basis_coef){
		lagrange_basis_coef[ix] = 1
		for i := range(points) {
			if i == ix {
				continue
			}
			part_numerator := points[i].x
			part_denominator := add(points[ix].x, points[i].x)
//			m.logger.Debug("", zap.Int("coef", ix), zap.Int("part", i), zap.Uint8("part num", part_numerator), zap.Uint8("part_denom", part_denominator))
			lagrange_basis_coef[ix] = mul(lagrange_basis_coef[ix], div(part_numerator, part_denominator))
		}
//		m.logger.Debug("", zap.Uint8("lagrange coef", lagrange_basis_coef[ix]))
//		m.logger.Debug("", zap.Uint8("y * lagrange_coef", mul(points[ix].y, lagrange_basis_coef[ix])))
//		m.logger.Debug("\n")
		ss = add(ss, mul(points[ix].y, lagrange_basis_coef[ix]))
	}

	return ss
}

func test(k, n byte, times int) {
	for t:=0; t < times; t++ {
		rand.Seed(time.Now().UnixNano())

		poly := make([]byte, k) //create a random polynom
		for i := range(poly){
			poly[i] = byte(rand.Intn(256))
		}

		secret := poly[0] //note its secret
		m := NewManager(k, n, secret)
		m.poly = poly

		points := make([]point, m.n) //evaluate polynom at points 1 to n
		for ix := range(points){ //points go from 1 onwards, because poly(0) = secret !
			points[ix].x = byte(1+ix)
			points[ix].y = m.poly_eval(points[ix].x)
		}

		//create random subset of points to use in interpolation
		points_subset := make([]point, m.k)
		for i, point_ix := range rand.Perm(int(m.n)){
			if i == int(m.k) {
				break
			}
			points_subset[i] = points[point_ix]
		}

		ss := m.interpolate(points_subset) //interpolate the secret
		if ss != secret {
			panic("interpolated secret does not equal original")
		}
	}
}


func main() {
	fmt.Println(mul(5, 7))

	k, n := byte(5), byte(15)
	test(k, n, 100000)
}
