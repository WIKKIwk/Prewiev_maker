package preview

type NamedOption struct {
	Key  string
	Name string
}

func ProductCategories() []NamedOption {
	order := []string{
		"",
		"electronics",
		"beauty",
		"beverage",
		"food",
		"home_living",
		"fashion",
		"luxury_object",
	}

	out := make([]NamedOption, 0, len(order))
	for _, key := range order {
		if pt, ok := productTypes[key]; ok {
			out = append(out, NamedOption{Key: key, Name: pt.Name})
		}
	}
	return out
}

func VisualStyles() []NamedOption {
	order := []string{
		"",
		"luxury_editorial",
		"minimal_museum",
		"futuristic_tech",
		"organic_sensory",
		"dark_premium",
		"high_key_clean",
		"monochrome_graphic",
		"brutalist_lux",
		"neo_pop_premium",
		"cinematic_film",
		"glass_light",
		"liquid_sculpture",
		"macro_lab",
		"gulliver_mini_workers",
		"fashion_editorial",
		"sports_energy_clean",
		"fantasy_surreal",
		"gold",
	}

	out := make([]NamedOption, 0, len(order))
	out = append(out, NamedOption{Key: "", Name: "Default"})
	for _, key := range order[1:] {
		if v, ok := visualPresets[key]; ok {
			out = append(out, NamedOption{Key: key, Name: v.Name})
		}
	}
	return out
}

func FrameTemplates() []FrameTemplate {
	out := make([]FrameTemplate, 0, len(frameTemplates))
	for _, t := range frameTemplates {
		out = append(out, cloneFrameTemplate(t))
	}
	return out
}
