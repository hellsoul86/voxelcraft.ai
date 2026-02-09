package world

func (w *World) ensureContainerForPlacedBlock(pos Vec3i, blockName string) {
	switch blockName {
	case "CHEST", "FURNACE", "CONTRACT_TERMINAL":
		w.ensureContainer(pos, blockName)
	}
}

func (w *World) ensureContainer(pos Vec3i, typ string) *Container {
	c := w.containers[pos]
	if c != nil {
		// If the type changed (shouldn't happen), overwrite.
		c.Type = typ
		return c
	}
	c = &Container{
		Type:      typ,
		Pos:       pos,
		Inventory: map[string]int{},
	}
	w.containers[pos] = c
	return c
}

func (w *World) removeContainer(pos Vec3i) *Container {
	c := w.containers[pos]
	if c == nil {
		return nil
	}
	delete(w.containers, pos)
	return c
}

func (w *World) getContainerByID(id string) *Container {
	typ, pos, ok := parseContainerID(id)
	if !ok {
		return nil
	}
	c := w.containers[pos]
	if c == nil {
		return nil
	}
	if c.Type != typ {
		return nil
	}
	return c
}

func (w *World) canWithdrawFromContainer(agentID string, pos Vec3i) bool {
	land := w.landAt(pos)
	if land == nil {
		return true
	}
	return w.isLandMember(agentID, land)
}
