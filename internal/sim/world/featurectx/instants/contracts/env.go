package contracts

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

type Env struct {
	GetContainerByIDFn     func(id string) *modelpkg.Container
	DistanceFn             func(a modelpkg.Vec3i, b modelpkg.Vec3i) int
	NewContractIDFn        func() string
	PutContractFn          func(c *modelpkg.Contract)
	GetContractFn          func(contractID string) *modelpkg.Contract
	RepDepositMultiplierFn func(a *modelpkg.Agent) int
	CheckBuildContractFn   func(c *modelpkg.Contract) bool
}

func (e Env) GetContainerByID(id string) *modelpkg.Container {
	if e.GetContainerByIDFn == nil {
		return nil
	}
	return e.GetContainerByIDFn(id)
}

func (e Env) Distance(a modelpkg.Vec3i, b modelpkg.Vec3i) int {
	if e.DistanceFn == nil {
		return 0
	}
	return e.DistanceFn(a, b)
}

func (e Env) NewContractID() string {
	if e.NewContractIDFn == nil {
		return ""
	}
	return e.NewContractIDFn()
}

func (e Env) PutContract(c *modelpkg.Contract) {
	if e.PutContractFn != nil {
		e.PutContractFn(c)
	}
}

func (e Env) GetContract(contractID string) *modelpkg.Contract {
	if e.GetContractFn == nil {
		return nil
	}
	return e.GetContractFn(contractID)
}

func (e Env) RepDepositMultiplier(a *modelpkg.Agent) int {
	if e.RepDepositMultiplierFn == nil {
		return 1
	}
	return e.RepDepositMultiplierFn(a)
}

func (e Env) CheckBuildContract(c *modelpkg.Contract) bool {
	if e.CheckBuildContractFn == nil {
		return false
	}
	return e.CheckBuildContractFn(c)
}
