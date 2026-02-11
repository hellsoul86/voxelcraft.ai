package world

import (
	"fmt"

	"voxelcraft.ai/internal/sim/catalogs"
)

func New(cfg WorldConfig, cats *catalogs.Catalogs) (*World, error) {
	cfg.applyDefaults()
	if cfg.Height != 1 {
		return nil, fmt.Errorf("2D world requires height=1 (got %d)", cfg.Height)
	}
	if err := validateActionDispatchMaps(); err != nil {
		return nil, err
	}

	smeltByInput, err := buildSmeltByInput(cats.Recipes.ByID)
	if err != nil {
		return nil, err
	}

	// Resolve required block ids.
	b := func(id string) (uint16, error) {
		v, ok := cats.Blocks.Index[id]
		if !ok {
			return 0, fmt.Errorf("missing block id in palette: %s", id)
		}
		return v, nil
	}
	air, _ := b("AIR")
	dirt, _ := b("DIRT")
	grass, _ := b("GRASS")
	sand, _ := b("SAND")
	stone, _ := b("STONE")
	gravel, _ := b("GRAVEL")
	logBlock, _ := b("LOG")
	coal, _ := b("COAL_ORE")
	iron, _ := b("IRON_ORE")
	copper, _ := b("COPPER_ORE")
	crystal, _ := b("CRYSTAL_ORE")

	gen := WorldGen{
		Seed:      cfg.Seed,
		BoundaryR: cfg.BoundaryR,
		// Worldgen tuning.
		BiomeRegionSize:                 cfg.BiomeRegionSize,
		SpawnClearRadius:                cfg.SpawnClearRadius,
		OreClusterProbScalePermille:     cfg.OreClusterProbScalePermille,
		TerrainClusterProbScalePermille: cfg.TerrainClusterProbScalePermille,
		SprinkleStonePermille:           cfg.SprinkleStonePermille,
		SprinkleDirtPermille:            cfg.SprinkleDirtPermille,
		SprinkleLogPermille:             cfg.SprinkleLogPermille,
		Air:                             air,
		Dirt:                            dirt,
		Grass:                           grass,
		Sand:                            sand,
		Stone:                           stone,
		Gravel:                          gravel,
		Log:                             logBlock,
		CoalOre:                         coal,
		IronOre:                         iron,
		CopperOre:                       copper,
		CrystalOre:                      crystal,
	}

	w := &World{
		cfg:           cfg,
		catalogs:      cats,
		chunks:        NewChunkStore(gen),
		smeltByInput:  smeltByInput,
		agents:        map[string]*Agent{},
		clients:       map[string]*clientState{},
		claims:        map[string]*LandClaim{},
		containers:    map[Vec3i]*Container{},
		items:         map[string]*ItemEntity{},
		itemsAt:       map[Vec3i][]string{},
		conveyors:     map[Vec3i]ConveyorMeta{},
		switches:      map[Vec3i]bool{},
		trades:        map[string]*Trade{},
		boards:        map[string]*Board{},
		signs:         map[Vec3i]*Sign{},
		contracts:     map[string]*Contract{},
		laws:          map[string]*Law{},
		orgs:          map[string]*Organization{},
		inbox:         make(chan ActionEnvelope, 1024),
		join:          make(chan JoinRequest, 64),
		attach:        make(chan AttachRequest, 64),
		admin:         make(chan adminSnapshotReq, 16),
		adminReset:    make(chan adminResetReq, 16),
		agentPosReq:   make(chan agentPosReq, 64),
		orgMetaReq:    make(chan orgMetaReq, 64),
		orgMetaUpsert: make(chan orgMetaUpsertReq, 64),
		leave:         make(chan string, 64),
		stop:          make(chan struct{}),
		transferOut:   make(chan transferOutReq, 64),
		transferIn:    make(chan transferInReq, 64),
		injectEvent:   make(chan injectEventReq, 256),
		observerJoin:  make(chan ObserverJoinRequest, 16),
		observerSub:   make(chan ObserverSubscribeRequest, 64),
		observerLeave: make(chan string, 16),
		weather:       "CLEAR",
		stats:         NewWorldStats(300, 72000),
		structures:    map[string]*Structure{},
		observers:     map[string]*observerClient{},
		resourceDensity: map[string]float64{
			"COAL_ORE":    0,
			"IRON_ORE":    0,
			"COPPER_ORE":  0,
			"CRYSTAL_ORE": 0,
			"STONE":       0,
			"LOG":         0,
		},
	}
	return w, nil
}
