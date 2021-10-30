package main
import (
	"fmt"
	_"math/rand"
	_"strconv"
	_"sync"
	_"time"

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
		m.logger.Debug("", zap.Uint8("coef", coef), zap.Uint8("value here", mul(coef, powered_x)))
		value = add(value, mul(coef, powered_x))
		m.logger.Debug("", zap.Uint8("coef", coef), zap.Uint8("total value here", value))
	}

	m.logger.Debug("", zap.Uint8("x", x), zap.Uint8("poly evaluated at x", value))
	m.logger.Debug("\n\n")
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
			fmt.Println(points[i])
			m.logger.Debug("", zap.Int("coef", ix), zap.Int("part", i), zap.Uint8("part num", part_numerator), zap.Uint8("part_denom", part_denominator))
			lagrange_basis_coef[ix] = mul(lagrange_basis_coef[ix], div(part_numerator, part_denominator))
		}
		m.logger.Debug("", zap.Uint8("lagrange coef", lagrange_basis_coef[ix]))
		m.logger.Debug("", zap.Uint8("y * lagrange_coef", mul(points[ix].y, lagrange_basis_coef[ix])))
		m.logger.Debug("\n")
		ss = add(ss, mul(points[ix].y, lagrange_basis_coef[ix]))
	}

	return ss
}

func main() {

	fmt.Println(div(12, 10))
	fmt.Println(div(4, 14))
	fmt.Println(div(3, 27))

	m := NewManager(3, 4, 17)

	points := make([]point, m.n)
	for ix := range(points){ //points go from 1 onwards, because poly(0) = secret !
		points[ix].x = byte(1+ix)
		points[ix].y = m.poly_eval(points[ix].x)
	}

	points_subset := []point{points[2], points[1], points[0]}
	ss := m.interpolate(points_subset)
	fmt.Println("\n\n", ss)
}
