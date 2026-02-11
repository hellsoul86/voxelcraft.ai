package ids

import (
	"fmt"
	"strconv"
	"strings"
)

func ContainerID(typ string, x, y, z int) string {
	return fmt.Sprintf("%s@%d,%d,%d", typ, x, y, z)
}

func ParseContainerID(id string) (typ string, x, y, z int, ok bool) {
	parts := strings.SplitN(id, "@", 2)
	if len(parts) != 2 {
		return "", 0, 0, 0, false
	}
	typ = parts[0]
	coord := strings.Split(parts[1], ",")
	if len(coord) != 3 {
		return "", 0, 0, 0, false
	}
	x, err1 := strconv.Atoi(coord[0])
	y, err2 := strconv.Atoi(coord[1])
	z, err3 := strconv.Atoi(coord[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return "", 0, 0, 0, false
	}
	return typ, x, y, z, true
}

func SignIDAt(x, y, z int) string {
	return ContainerID("SIGN", x, y, z)
}

func ConveyorIDAt(x, y, z int) string {
	return ContainerID("CONVEYOR", x, y, z)
}

func SwitchIDAt(x, y, z int) string {
	return ContainerID("SWITCH", x, y, z)
}
