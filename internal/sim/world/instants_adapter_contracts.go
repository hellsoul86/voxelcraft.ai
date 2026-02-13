package world

type contractInstantsWorldEnv struct {
	w *World
}

func (e contractInstantsWorldEnv) GetContainerByID(id string) *Container {
	if e.w == nil {
		return nil
	}
	return e.w.getContainerByID(id)
}

func (e contractInstantsWorldEnv) Distance(a Vec3i, b Vec3i) int {
	return Manhattan(a, b)
}

func (e contractInstantsWorldEnv) NewContractID() string {
	if e.w == nil {
		return ""
	}
	return e.w.newContractID()
}

func (e contractInstantsWorldEnv) PutContract(c *Contract) {
	if e.w == nil || c == nil {
		return
	}
	e.w.contracts[c.ContractID] = c
}

func (e contractInstantsWorldEnv) GetContract(contractID string) *Contract {
	if e.w == nil {
		return nil
	}
	return e.w.contracts[contractID]
}

func (e contractInstantsWorldEnv) RepDepositMultiplier(a *Agent) int {
	if e.w == nil {
		return 1
	}
	return e.w.repDepositMultiplier(a)
}

func (e contractInstantsWorldEnv) CheckBuildContract(c *Contract) bool {
	if e.w == nil || c == nil {
		return false
	}
	buildOK := e.w.checkBlueprintPlaced(c.BlueprintID, c.Anchor, c.Rotation)
	if !buildOK {
		return false
	}
	bp, ok := e.w.catalogs.Blueprints.ByID[c.BlueprintID]
	if !ok {
		return false
	}
	return e.w.structureStable(&bp, c.Anchor, c.Rotation)
}
