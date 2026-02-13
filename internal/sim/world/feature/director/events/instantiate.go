package events

type SpawnKind string

const (
	SpawnNone         SpawnKind = ""
	SpawnCrystalRift  SpawnKind = "CRYSTAL_RIFT"
	SpawnDeepVein     SpawnKind = "DEEP_VEIN"
	SpawnRuinsGate    SpawnKind = "RUINS_GATE"
	SpawnFloodWarning SpawnKind = "FLOOD_WARNING"
	SpawnBanditCamp   SpawnKind = "BANDIT_CAMP"
	SpawnBlightZone   SpawnKind = "BLIGHT_ZONE"
	SpawnNoticeBoard  SpawnKind = "NOTICE_BOARD"
)

type InstantiatePlan struct {
	NeedsCenter bool
	Radius      int
	Spawn       SpawnKind
	Headline    string
	Body        string
}

func BuildInstantiatePlan(eventID string, params map[string]any) InstantiatePlan {
	switch eventID {
	case "CRYSTAL_RIFT":
		return InstantiatePlan{NeedsCenter: true, Radius: 32, Spawn: SpawnCrystalRift}
	case "DEEP_VEIN":
		return InstantiatePlan{NeedsCenter: true, Radius: 40, Spawn: SpawnDeepVein}
	case "RUINS_GATE":
		return InstantiatePlan{NeedsCenter: true, Radius: 24, Spawn: SpawnRuinsGate}
	case "MARKET_WEEK":
		return InstantiatePlan{
			NeedsCenter: true,
			Radius:      32,
			Spawn:       SpawnNoticeBoard,
			Headline:    "市集周",
			Body:        "市场税临时减免，鼓励交易与签约。",
		}
	case "BLUEPRINT_FAIR":
		return InstantiatePlan{
			NeedsCenter: true,
			Radius:      32,
			Spawn:       SpawnNoticeBoard,
			Headline:    "蓝图开放日",
			Body:        "分享与复用蓝图将获得额外影响力。",
		}
	case "BUILDER_EXPO":
		theme := "MONUMENT"
		if v, ok := params["theme"]; ok {
			if s, ok := v.(string); ok && s != "" {
				theme = s
			}
		}
		return InstantiatePlan{
			NeedsCenter: true,
			Radius:      40,
			Spawn:       SpawnNoticeBoard,
			Headline:    "建筑大赛",
			Body:        "主题: " + theme + "。完成蓝图建造并展示。",
		}
	case "FLOOD_WARNING":
		return InstantiatePlan{
			NeedsCenter: true,
			Radius:      40,
			Spawn:       SpawnFloodWarning,
			Headline:    "洪水风险",
			Body:        "低地可能被淹，建议修堤坝与迁移仓库。",
		}
	case "BANDIT_CAMP":
		return InstantiatePlan{NeedsCenter: true, Radius: 24, Spawn: SpawnBanditCamp}
	case "BLIGHT_ZONE":
		return InstantiatePlan{
			NeedsCenter: true,
			Radius:      32,
			Spawn:       SpawnBlightZone,
			Headline:    "污染扩散",
			Body:        "在污染区行动会降低体力恢复并加速饥饿。",
		}
	case "CIVIC_VOTE":
		return InstantiatePlan{
			NeedsCenter: true,
			Radius:      32,
			Spawn:       SpawnNoticeBoard,
			Headline:    "城邦选举/公投",
			Body:        "提出法律并投票将获得额外叙事分。",
		}
	default:
		return InstantiatePlan{}
	}
}
