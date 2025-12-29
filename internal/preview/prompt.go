package preview

import (
	"fmt"
	"strconv"
	"strings"
)

type Options struct {
	Mode          string // "grid" | "vertical"
	GridPreset    string // "1x1" | "2x2" | "3x2" | "3x3"
	VerticalCount string // "1" | "2" | "3" | "4"
	AspectRatio   string // optional override, e.g. "1:1", "4:5", "9:16"
	FrameIDs      []string
	ProductType   string // "" | "electronics" | "beauty" | ...
	VisualStyle   string // "" | "dark_premium" | ...
	HumanUsage    bool
	Custom        string
}

type OutputPreset struct {
	Mode            string
	Cols            int
	Rows            int
	Count           int
	AspectRatio     string
	ResolutionHint  string
	LayoutPresetKey string
}

type FrameTemplate struct {
	ID        string
	Title     string
	Concept   string
	Execution []string
}

type ProductType struct {
	Name   string
	Global []string
	Frame3 []string
	Frame8 []string
}

type VisualPreset struct {
	Name  string
	Add   []string
	Notes []string
}

const (
	aspectGrid     = "3:4"
	aspectVertical = "9:16"
)

var gridPresets = map[string]struct{ Cols, Rows int }{
	"1x1": {Cols: 1, Rows: 1},
	"2x2": {Cols: 2, Rows: 2},
	"3x2": {Cols: 3, Rows: 2},
	"3x3": {Cols: 3, Rows: 3},
}

var verticalCounts = map[string]int{
	"1": 1,
	"2": 2,
	"3": 3,
	"4": 4,
}

var frameTemplates = []FrameTemplate{
	{
		ID:      "hero_still_life",
		Title:   "Iconic Hero Still Life",
		Concept: "Bold, confident product presentation with dramatic composition",
		Execution: []string{
			"Center-framed product on seamless background",
			"Strong directional key light from 45° angle",
			"Deep shadows for depth and dimension",
			"Negative space emphasizing product authority",
			"Ultra-sharp focus, every detail visible",
			"Color grading: rich, saturated, premium feel",
		},
	},
	{
		ID:      "extreme_macro",
		Title:   "Extreme Macro Detail",
		Concept: "Surface texture and material craftsmanship",
		Execution: []string{
			"Hyper-close-up on product surface/texture (and authentic label/markings if present)",
			"Shallow depth of field, bokeh background",
			"Reveal material quality: glass reflection, paper fiber, metal grain",
			"Macro lens precision",
			"Highlight authentic typography/markings if present, otherwise focus on unique material details",
			"Clinical sharpness in focused area",
		},
	},
	{
		ID:      "dynamic_interaction",
		Title:   "Dynamic Particle Interaction",
		Concept: "Product surrounded by motion and energy",
		Execution: []string{
			"Particle cloud, powder burst, light streaks, or category-appropriate micro-effects around product (environment only)",
			"Product remains perfectly still and centered",
			"Frozen motion capture (high-speed photography aesthetic)",
			"Effects complement product color palette",
			"Controlled chaos: dynamic yet clean",
			"Avoid liquids unless the product category clearly implies it",
			"Product untouched, pristine",
		},
	},
	{
		ID:      "minimal_sculptural",
		Title:   "Minimal Sculptural Arrangement",
		Concept: "Abstract forms meeting product design",
		Execution: []string{
			"Product placed among geometric shapes (spheres, cubes, cylinders)",
			"Monochromatic or tonal color scheme",
			"Architectural precision in object placement",
			"Clean lines, perfect symmetry or intentional asymmetry",
			"Matte and glossy surface interplay",
			"Museum-quality lighting",
		},
	},
	{
		ID:      "floating_elements",
		Title:   "Floating Elements Composition",
		Concept: "Weightlessness, innovation, future-forward",
		Execution: []string{
			"Product appears to levitate",
			"Supporting elements suspended mid-air (ribbons/fabrics/petals/components as abstract cues)",
			"Invisible support wires aesthetic",
			"Airy, light-filled environment",
			"Soft shadows suggesting gentle elevation",
			"Ethereal yet grounded in realism",
		},
	},
	{
		ID:      "sensory_closeup",
		Title:   "Sensory Close-Up",
		Concept: "Tactile invitation, almost touchable realism",
		Execution: []string{
			"Tight crop emphasizing product shape and form",
			"Lighting that reveals three-dimensionality",
			"Focus on how light plays across the surface",
			"Viewer feels texture through the image",
			"Intimate perspective",
			"Warm, inviting atmosphere",
		},
	},
	{
		ID:      "precision_feature_study",
		Title:   "Precision Detail Feature Study",
		Concept: "Premium micro-details and feature craftsmanship (not just texture)",
		Execution: []string{
			"Medium-macro close-up that still shows the product’s form (not an abstract texture-only crop).",
			"Focus on a signature feature: edge bevel, cap mechanism, nozzle, embossing, seam/stitch, button/knurl, hinge, or material junction.",
			"Raking side light to reveal precision; controlled specular highlights.",
			"Focus-stacking look (sharp across the key feature) while background falls to soft bokeh.",
			"Show fit-and-finish; zero dust, zero fingerprints.",
			"Keep branding accurate and legible where visible; never crop in a way that changes perceived logo/typography.",
		},
	},
	{
		ID:      "ingredient_abstraction",
		Title:   "Ingredient/Component Abstraction",
		Concept: "Symbolic representation, not literal",
		Execution: []string{
			"Build a symbolic, non-literal abstraction of the product’s essence (component/ingredient vibe, not a literal pile).",
			"Use refined material metaphors: textures, silhouettes, micro-forms, geometric cues.",
			"Keep the product as the hero; abstraction supports it without competing.",
			"Commercial clarity: premium, clean, immediately readable as high-end advertising.",
			"No gimmicks, no clutter, no readable text; avoid kitschy literal props.",
			"Maintain consistent studio-grade lighting and pristine product integrity.",
		},
	},
	{
		ID:      "surreal_fusion",
		Title:   "Surreal Elegant Fusion",
		Concept: "Reality meets imagination, unexpected yet harmonious",
		Execution: []string{
			"Product in impossible but beautiful scenario",
			"Floating in cloud-like softness, or reflective infinity space",
			"Dreamlike distortion in environment only (never distort product)",
			"Product remains photographically accurate",
			"Surrealism in setting, realism in product",
			"High fashion editorial meets fine art",
		},
	},
}

var productTypes = map[string]ProductType{
	"": {
		Name: "Auto/General",
		Global: []string{
			"Choose category-appropriate interactions that never alter the product.",
			"Avoid literal ingredients unless clearly implied by the product itself.",
		},
		Frame3: []string{
			"Use particles, clean light streaks, or gentle atmospheric haze as an abstract energy accent (environment only).",
			"Keep effects behind/beside the product; never cover key details or any real branding.",
		},
		Frame8: []string{
			"Use symbolic material cues that suggest components/essence (non-literal).",
			"Keep it refined, minimal, and commercially clear.",
		},
	},
	"electronics": {
		Name: "Electronics / Tech",
		Global: []string{
			"Tech cues must come from lighting, precision surfaces, and abstract geometry—not busy UI graphics.",
			"No readable UI or circuitry text.",
		},
		Frame3: []string{
			"Use controlled micro-particles, ionized mist, or clean light streaks (environment only).",
			"Avoid messy liquid; prefer precision energy effects.",
		},
		Frame8: []string{
			"Abstract components: prismatic glass, anodized metal fragments, micro-lens bokeh, clean electromagnetic lines (non-text).",
			"Symbolize performance/precision without literal parts.",
		},
	},
	"beauty": {
		Name: "Beauty / Cosmetic",
		Global: []string{
			"Sensory softness and tactile finish are key; keep it premium and clean.",
			"No rendered claims as text.",
		},
		Frame3: []string{
			"Use silk-like powder bloom, fine mist, pearlescent micro-particles, or viscous glossy gel arcs.",
			"Controlled chaos; keep packaging pristine.",
		},
		Frame8: []string{
			"Abstract essence: mineral textures, botanical silhouettes, creamy swirls, translucent petals (symbolic, not literal).",
			"Commercial clarity with refined artistry.",
		},
	},
	"beverage": {
		Name: "Beverage",
		Global: []string{
			"Emphasize coldness, freshness, and clarity; keep label readable.",
			"Condensation/ice cues must look physically correct.",
		},
		Frame3: []string{
			"Use sculptural liquid splash arcs, micro-droplets, and ice crystals framing the product.",
			"High-speed frozen motion aesthetic; no label occlusion.",
		},
		Frame8: []string{
			"Abstract components: ice formations, botanical silhouettes, carbonation bubbles, liquid droplets (symbolic).",
			"No literal fruit piles unless the product explicitly implies it.",
		},
	},
	"food": {
		Name: "Food / Gourmet",
		Global: []string{
			"Keep it appetizing but still luxury editorial; avoid messy crumbs unless controlled.",
			"No readable menu text or overlays.",
		},
		Frame3: []string{
			"Use fine spice/powder burst, steam-like haze, or crisp particle motion (controlled).",
			"If liquid is used, keep it minimal and sculptural.",
		},
		Frame8: []string{
			"Abstract essence: refined textures (salt crystals, cocoa dust, grain patterns) in geometric composition, not literal piles.",
			"Symbolic, minimal, premium.",
		},
	},
	"home_living": {
		Name: "Home & Living",
		Global: []string{
			"Emphasize material honesty (wood/ceramic/textile cues) and calm premium atmosphere.",
			"Keep environment uncluttered and gallery-like.",
		},
		Frame3: []string{
			"Use gentle dust motes, clean fabric ribbon motion, or soft particles—subtle, not energetic.",
			"Maintain serene, controlled composition.",
		},
		Frame8: []string{
			"Abstract components: textile weaves, ceramic glaze textures, soft natural shapes (non-literal).",
			"Suggest comfort and quality through material cues.",
		},
	},
	"fashion": {
		Name: "Fashion / Accessories",
		Global: []string{
			"Editorial fashion sensibility; shape, silhouette, and light are the hero.",
			"Avoid any readable magazine text or extra logos.",
		},
		Frame3: []string{
			"Use flowing fabric-like motion, light ribbons, or minimal particles that feel runway/editorial.",
			"Keep it elegant and restrained.",
		},
		Frame8: []string{
			"Abstract components: leather grain, metal hardware reflections, textile fibers, gemstone-like bokeh (symbolic).",
			"Non-obvious, high-fashion refinement.",
		},
	},
	"luxury_object": {
		Name: "Luxury Object",
		Global: []string{
			"Museum-grade restraint: precious materials, pristine reflections, controlled sparkle.",
			"No gaudy glints; highlight craft and rarity.",
		},
		Frame3: []string{
			"Use subtle luminous dust, refined micro-sparkle, or elegant haze—not chaotic splashes.",
			"Keep it premium and quiet.",
		},
		Frame8: []string{
			"Abstract essence: gemstone refractions, brushed metal micro-texture, incense-like wisps (symbolic).",
			"Fine-art energy with commercial clarity.",
		},
	},
}

var visualPresets = map[string]VisualPreset{
	"gulliver_mini_workers": {
		Name: "Gulliver Miniature Workers (Diorama)",
		Add: []string{
			"gulliver-scale diorama: the product is a giant monument, tiny workers interact with it",
			"miniature people (tiny engineers/riggers/technicians) working around the product",
			"tiny ropes, pulleys, scaffolding, ladders, miniature cranes (props only)",
			"micro-scale construction/maintenance scene: polishing, measuring, inspecting, securing",
			"macro photography of a miniature diorama, realistic scale cues",
			"tilt-shift miniature look (subtle), shallow DOF with controlled focus plane",
			"premium high-end advertising finish (clean, intentional, not messy)",
			"product remains 100% photorealistic and undistorted",
			"do not cover label/branding; keep it readable where visible",
			"no readable text anywhere in the scene",
		},
		Notes: []string{
			"Gulliver rule: tiny workers + giant product, but still premium and clean.",
			"Workers/props must never block or damage branding/label.",
		},
	},
	"gold": {
		Name: "Opulent Gold (Indulgent Luxury)",
		Add: []string{
			"Use the attached reference photo as the exact product identity lock.",
			"Interpret and replicate the product from the reference precisely: shape, proportions, silhouette, materials, finishes, colors, logos/decals, knobs/switches, hardware placement.",
			"Do not redesign, do not change the model, and do not introduce extra parts.",
			"indulgent luxury product photography, opulent richness, wealth aesthetic",
			"bathed in warm golden light (honey-toned highlights)",
			"warm highlights and deep shadows, cinematic contrast",
			"premium studio lighting, controlled reflections, high-end commercial luxury advertising look",
			"surrounded by gold leaf flakes and fine gold dust micro-particles (subtle, premium, not chaotic)",
			"honey-toned reflective surfaces / glossy dark surfaces as environment only",
			"hero product centered, clean separation from background, crisp edges",
			"shallow depth of field, ultra-detailed photoreal, tack-sharp product focus",
			"background stays elegant and minimal; gold elements frame the product without clutter",
			"no text, no watermark, no border, no frame, no bars",
			"full-bleed image: no empty edges; do not add padding or letterboxing",
			"gold effects must NOT cover label/branding; keep typography readable where visible",
			"never recolor or alter the product; gold tone applies to lighting/environment only",
		},
		Notes: []string{
			"Gold style = lighting + environment + micro-particles. Product identity stays 100% locked to reference.",
			"Keep it 'rich and controlled' (no cheap glitter, no party confetti).",
		},
	},
	"neo_pop_premium": {
		Name: "Neo-Pop (Pop Star / Stage)",
		Add: []string{
			"pop star stage vibe (premium pop aesthetic)",
			"bold pop color blocking (background/set only)",
			"neon accent lighting, clean rim lights (controlled, not chaotic)",
			"graphic shapes: circles, stripes, geometric cutouts (set design only)",
			"glossy acrylic / chrome props (background only), modern pop set",
			"high-saturation accents with strict restraint (2–3 accent colors max)",
			"clean gradients, crisp edges, studio-grade polish",
			"sparkle micro-particles / confetti hints (very subtle, premium)",
			"product remains photorealistic, pristine, tack-sharp",
		},
		Notes: []string{
			"Accent colors apply to environment only; NEVER recolor the product.",
		},
	},
	"luxury_editorial": {
		Name: "Luxury Editorial (Gala Awards)",
		Add: []string{
			"ultra-premium luxury editorial advertising",
			"award gala atmosphere (high-end ceremony vibe)",
			"black & gold stage palette (environment only)",
			"cinematic spotlight beams, controlled volumetric haze",
			"gold confetti micro-particles / stardust dust (subtle, premium)",
			"luxury bokeh light wall (background only)",
			"museum-grade product realism: pristine, perfect reflections",
		},
		Notes: []string{
			"Gala mood: elegant ceremony lighting + subtle gold dust, NOT party chaos.",
		},
	},
	"minimal_museum": {
		Name: "Miniature Museum (Diorama)",
		Add: []string{
			"miniature museum diorama set (scale model exhibition space)",
			"macro photography of a miniature diorama (tiny set details visible)",
			"tilt-shift miniature look (subtle), shallow depth of field with controlled focus plane",
			"museum-grade minimalism (few elements)",
			"product remains photorealistic, pristine, and undistorted",
			"no readable text anywhere in the scene",
		},
		Notes: []string{
			"Miniature rule: the environment is a scale model museum diorama.",
		},
	},
	"dark_premium": {
		Name:  "Dark Premium",
		Add:   []string{"dark premium advertising", "low-key studio lighting", "controlled rim light", "deep gradients"},
		Notes: []string{"Label must remain readable on dark."},
	},
	"high_key_clean": {
		Name:  "High-Key Clean",
		Add:   []string{"high-key bright studio", "clean white/ivory backgrounds", "soft shadow under product", "clinical clarity"},
		Notes: []string{"Avoid blown highlights; keep micro-detail."},
	},
	"futuristic_tech": {
		Name:  "Futuristic Tech",
		Add:   []string{"futuristic premium tech advertising", "sterile clean studio", "precision lighting", "high-contrast micro-detail"},
		Notes: []string{"No cyber clutter; premium minimal."},
	},
	"glass_light": {
		Name: "Glass & Light (Clean Tech Ad)",
		Add: []string{
			"glass-first environment: transparent glass, crystal, acrylic, and prism slabs (background only)",
			"clean caustics patterns on the floor/walls (subtle, realistic)",
			"prismatic light rays, controlled rainbow dispersion (very refined)",
			"NO refraction passing through the product silhouette",
			"product must remain perfectly undistorted and photorealistic",
		},
		Notes: []string{
			"Refraction/glass effects must frame the product, never warp label/typography.",
		},
	},
	"macro_lab": {
		Name: "Macro Lab (Detail-Only)",
		Add: []string{
			"precision macro lab vibe",
			"detail-only macro photography: show only product micro-details, not a full product hero shot",
			"extreme macro framing of label print, embossing, seam, edge bevel, cap mechanism, nozzle, texture, material junction",
			"background must be mid-gray to dark-gray neutral gradient (graphite/charcoal), NOT white",
			"clinical clarity, zero dust, zero fingerprints",
		},
		Notes: []string{
			"Detail-only rule: tight macro close-ups; avoid wide shots.",
		},
	},
	"monochrome_graphic": {
		Name: "Japanese B&W Cinema (Premium)",
		Add: []string{
			"true black-and-white cinematography (no color)",
			"high-contrast lighting, deep blacks, rich midtones",
			"subtle 35mm film grain (fine, premium)",
		},
		Notes: []string{
			"FULL BLACK-AND-WHITE LOOK: the entire image should be monochrome.",
		},
	},
	"brutalist_lux": {
		Name:  "Brutalist Luxury",
		Add:   []string{"brutalist luxury set design", "raw stone/concrete vibe (background only)", "hard geometry with soft light"},
		Notes: []string{"Abstract props; keep it clean."},
	},
	"liquid_sculpture": {
		Name: "Liquid Sculpture (Wrap / Orbit)",
		Add: []string{
			"sculptural liquid ribbons wrapping around the product (360-degree orbit)",
			"liquid arcs / rings framing the product from all sides",
			"never crossing the label/branding",
			"high-speed splash aesthetic with frozen motion (studio-grade)",
		},
		Notes: []string{
			"Never cover label/branding; keep product readable and undistorted.",
		},
	},
	"organic_sensory": {
		Name: "Organic Sensory (Tactile Luxury)",
		Add: []string{
			"organic sensory luxury advertising",
			"soft window daylight in studio (diffused natural light)",
			"natural materials as set design only: linen fabric, matte ceramic, warm wood grain, soft stone",
			"calm premium mood, spa/boutique sensibility",
		},
		Notes: []string{
			"Keep it minimal and premium; product identity must never change.",
		},
	},
	"fashion_editorial": {
		Name:  "Fashion Editorial",
		Add:   []string{"fashion editorial lighting", "lookbook polish", "soft contrast"},
		Notes: []string{"Editorial taste; minimal but expressive."},
	},
	"sports_energy_clean": {
		Name: "Sports Energy Clean (High-Speed Action Cam)",
		Add: []string{
			"high-speed sports commercial photography",
			"fast-shutter action capture (crisp freeze + controlled motion accents)",
			"clean motion streaks made of light (environment only, premium)",
			"product remains photorealistic, pristine, and undistorted",
		},
		Notes: []string{
			"Energy effects are controlled and clean (no chaotic dust clouds).",
		},
	},
	"cinematic_film": {
		Name:  "Cinematic Film",
		Add:   []string{"cinematic filmic lighting", "subtle film grain", "controlled halation"},
		Notes: []string{"Filmic but still billboard-clean."},
	},
	"fantasy_surreal": {
		Name: "Fantasy (Surreal)",
		Add: []string{
			"fantasy surreal high-end advertising",
			"floating architecture / impossible geometry (background only)",
			"premium VFX particles: iridescent dust, aurora-like ribbons (environment only)",
			"surreal but elegant, never kitschy",
		},
		Notes: []string{
			"Surrealism is allowed ONLY in the environment; the product must remain perfectly realistic and undistorted.",
		},
	},
}

func ResolveOutputPreset(opts Options) OutputPreset {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	gridKey := strings.ToLower(strings.TrimSpace(opts.GridPreset))
	verticalKey := strings.ToLower(strings.TrimSpace(opts.VerticalCount))

	if mode == "vertical" {
		count, ok := verticalCounts[verticalKey]
		if !ok {
			count = verticalCounts["4"]
			verticalKey = "4"
		}

		out := OutputPreset{
			Mode:            "vertical",
			Cols:            1,
			Rows:            count,
			Count:           count,
			AspectRatio:     aspectVertical,
			ResolutionHint:  "4K",
			LayoutPresetKey: verticalKey + "_vertical_images",
		}
		if ar := normalizeAspectRatio(opts.AspectRatio); ar != "" {
			out.AspectRatio = ar
		}
		return out
	}

	preset, ok := gridPresets[gridKey]
	if !ok {
		preset = gridPresets["3x3"]
		gridKey = "3x3"
	}

	out := OutputPreset{
		Mode:            "grid",
		Cols:            preset.Cols,
		Rows:            preset.Rows,
		Count:           preset.Cols * preset.Rows,
		AspectRatio:     aspectGrid,
		ResolutionHint:  "4K",
		LayoutPresetKey: gridKey,
	}
	if ar := normalizeAspectRatio(opts.AspectRatio); ar != "" {
		out.AspectRatio = ar
	}
	return out
}

func FramesForCount(n int) []FrameTemplate {
	if n < 1 {
		n = 1
	}
	if n > len(frameTemplates) {
		n = len(frameTemplates)
	}
	out := make([]FrameTemplate, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, cloneFrameTemplate(frameTemplates[i]))
	}
	return out
}

func framesForOutput(count int, selectedIDs []string) []FrameTemplate {
	if len(selectedIDs) == 0 {
		return FramesForCount(count)
	}

	byID := make(map[string]FrameTemplate, len(frameTemplates))
	for _, t := range frameTemplates {
		byID[t.ID] = t
	}

	var out []FrameTemplate
	seen := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		if tpl, ok := byID[id]; ok {
			out = append(out, cloneFrameTemplate(tpl))
		}
	}

	if len(out) == 0 {
		return FramesForCount(count)
	}

	if len(out) > count {
		return out[:count]
	}

	if len(out) < count {
		for _, tpl := range frameTemplates {
			if len(out) >= count {
				break
			}
			if _, ok := seen[tpl.ID]; ok {
				continue
			}
			seen[tpl.ID] = struct{}{}
			out = append(out, cloneFrameTemplate(tpl))
		}
	}

	if len(out) > count {
		out = out[:count]
	}
	return out
}

func ParseArgs(raw string, defaults Options) Options {
	opts := defaults
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return opts
	}

	var custom []string
	for _, tok := range strings.Fields(raw) {
		orig := tok
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "" {
			continue
		}

		switch tok {
		case "grid", "horizontal", "h":
			opts.Mode = "grid"
			continue
		case "vertical", "portrait", "v":
			opts.Mode = "vertical"
			continue
		case "use", "human", "usage", "inuse":
			opts.HumanUsage = true
			continue
		case "nohuman", "nouse", "no-usage":
			opts.HumanUsage = false
			continue
		}

		if _, ok := gridPresets[tok]; ok {
			opts.Mode = "grid"
			opts.GridPreset = tok
			continue
		}
		if strings.HasPrefix(tok, "v") && len(tok) == 2 {
			if _, ok := verticalCounts[tok[1:]]; ok {
				opts.Mode = "vertical"
				opts.VerticalCount = tok[1:]
				continue
			}
		}
		if strings.HasPrefix(tok, "style=") {
			style := strings.TrimSpace(strings.TrimPrefix(tok, "style="))
			if _, ok := visualPresets[style]; ok {
				opts.VisualStyle = style
				continue
			}
		}
		if strings.HasPrefix(tok, "cat=") || strings.HasPrefix(tok, "category=") {
			key := tok
			key = strings.TrimPrefix(key, "category=")
			key = strings.TrimPrefix(key, "cat=")
			key = strings.TrimSpace(key)
			if _, ok := productTypes[key]; ok {
				opts.ProductType = key
				continue
			}
		}
		if _, ok := productTypes[tok]; ok {
			opts.ProductType = tok
			continue
		}
		if _, ok := visualPresets[tok]; ok {
			opts.VisualStyle = tok
			continue
		}
		if strings.HasPrefix(tok, "ar=") || strings.HasPrefix(tok, "aspect=") {
			ar := tok
			ar = strings.TrimPrefix(ar, "aspect=")
			ar = strings.TrimPrefix(ar, "ar=")
			if norm := normalizeAspectRatio(ar); norm != "" {
				opts.AspectRatio = norm
				continue
			}
		}
		if norm := normalizeAspectRatio(tok); norm != "" {
			opts.AspectRatio = norm
			continue
		}

		custom = append(custom, orig)
	}

	opts.Custom = strings.TrimSpace(strings.Join(custom, " "))
	return opts
}

func BuildPrompt(opts Options) (string, OutputPreset) {
	out := ResolveOutputPreset(opts)

	productTypeKey := strings.ToLower(strings.TrimSpace(opts.ProductType))
	productType, ok := productTypes[productTypeKey]
	if !ok {
		productType = productTypes[""]
		productTypeKey = ""
	}

	visualKey := strings.ToLower(strings.TrimSpace(opts.VisualStyle))
	visual, hasVisual := visualPresets[visualKey]

	frames := framesForOutput(out.Count, opts.FrameIDs)
	for i := range frames {
		frames[i].Execution = append(frames[i].Execution,
			"REFERENCE MOOD LOCK: match the reference image mood, lighting, contrast, and palette; avoid off-palette backgrounds/effects.",
		)
		if frames[i].ID == "dynamic_interaction" {
			frames[i].Execution = append(frames[i].Execution, productType.Frame3...)
		}
		if frames[i].ID == "ingredient_abstraction" {
			frames[i].Execution = append(frames[i].Execution, productType.Frame8...)
		}
		if out.Mode == "vertical" {
			frames[i].Execution = append(frames[i].Execution,
				"Portrait composition lock: "+out.AspectRatio+" framing.",
				"Use vertical negative space (top/bottom) for premium layout; product centered, hero scale consistent.",
				"If needed, extend background vertically (outpainting) rather than changing product shape or cropping branding.",
				"Full-bleed rule: extend background to the edges; never leave empty/solid-color borders.",
			)
		}
		if hasVisual {
			frames[i].Execution = append(frames[i].Execution,
				"STYLE ENFORCEMENT: "+visual.Name+" (follow selected visual style strictly).",
				"Do not alter product identity.",
			)
		}
		if opts.HumanUsage {
			frames[i].Execution = append(frames[i].Execution,
				"Include human interaction crop (hands/partial) with NO full face; keep premium editorial.",
				"Product stays tack-sharp and dominant; skin/hand is supporting element only.",
			)
		}
		frames[i].Execution = uniq(frames[i].Execution)
	}

	var b strings.Builder
	b.Grow(4096)

	b.WriteString("TASK: Premium marketplace-ready product preview generation.\n\n")

	b.WriteString("REFERENCE IMAGE (IDENTITY LOCK): The attached photo contains the real product. Treat this as an image-edit/compositing task.\n")
	b.WriteString("- The product in every output MUST be the exact same object from the reference photo.\n")
	b.WriteString("- Preserve shape, proportions, materials, colors, and all physical details exactly.\n")
	b.WriteString("- Do NOT replace the product with another item (no substitutions) or invent a different product type.\n")
	b.WriteString("- Branding/text rule: if the reference has text/logo/label, keep it exactly; if it has none, do NOT add any text/logo/brand/claims.\n")
	b.WriteString("- If the reference photo includes a room/background, isolate the main product and replace the background with the requested studio scene.\n")
	b.WriteString("- You may remove background and re-light; never redesign the product or add new parts.\n")
	b.WriteString("- Do NOT add captions/watermarks/text overlays.\n\n")

	b.WriteString("OUTPUT SPEC:\n")
	b.WriteString(fmt.Sprintf("- Create %d images.\n", out.Count))
	b.WriteString(fmt.Sprintf("- Aspect ratio per image: %s (%s).\n", out.AspectRatio, out.Mode))
	b.WriteString(fmt.Sprintf("- Quality: %s. Lighting: studio-grade.\n", out.ResolutionHint))
	b.WriteString("- FULL-BLEED REQUIRED: no borders/frames/bars/mattes/padding/margins/empty edges; if ratio mismatch, outpaint/extend background.\n\n")

	b.WriteString("DIRECTION (Jason-style high-end commercial marketing):\n")
	for _, line := range []string{
		"Clean. Controlled. Intentional.",
		"Every element serves the product.",
		"No decoration for decoration's sake.",
		"Precision in execution.",
		"Emotion through restraint.",
		"Premium through simplicity.",
		"Cinematic without being theatrical.",
		"Commercial but never compromising artistry.",
	} {
		b.WriteString("- " + line + "\n")
	}
	b.WriteString("\n")

	b.WriteString("UNIVERSAL TECHNICAL SPECS:\n")
	writeSection(&b, "Product integrity", []string{
		"100% accurate shape and proportions (same object as reference)",
		"No distortion, warping, redesign, or substitution",
		"Branding/text: if present in reference, keep legible and unchanged; if absent, add none",
		"Color-matched materials and finishes",
		"Pristine condition",
	})
	writeSection(&b, "Lighting", []string{"Soft, controlled studio setup", "Balanced key, fill, rim", "Subtle specular highlights", "Natural shadow falloff", "No harsh or unnatural lighting"})
	writeSection(&b, "Focus and detail", []string{"Tack-sharp on product (except intentional bokeh areas)", "High-resolution rendering", "Fine detail visible: texture, print, surface quality", "Professional depth-of-field control"})
	writeSection(&b, "Composition", []string{"Clean separation product/background", "Clear visual hierarchy", "Intentional negative space", "Balanced frame weight"})
	writeSection(&b, "Post production", []string{"HDR look", "Subtle color grading", "Minimal but precise retouching", "Editorial polish without over-processing", "Medium-format camera aesthetic"})
	writeSection(&b, "Aesthetic", []string{"Luxury brand campaign quality", "Sophisticated, modern, timeless", "Aspirational yet authentic"})
	b.WriteString("\n")

	b.WriteString("CATEGORY:\n")
	b.WriteString(fmt.Sprintf("- %s\n", productType.Name))
	for _, line := range productType.Global {
		b.WriteString("- " + line + "\n")
	}
	b.WriteString("\n")

	if hasVisual {
		b.WriteString("VISUAL STYLE (STRICT):\n")
		b.WriteString("- " + visual.Name + "\n")
		for _, line := range visual.Add {
			b.WriteString("- " + line + "\n")
		}
		for _, line := range visual.Notes {
			b.WriteString("- NOTE: " + line + "\n")
		}
		b.WriteString("\n")
	}

	if opts.HumanUsage {
		b.WriteString("HUMAN USAGE SCENE (ENFORCEMENT):\n")
		for _, line := range []string{
			"Include human interaction/usage context, but NEVER show a full face.",
			"No identifiable person: no eyes + nose + full face together; avoid portraits.",
			"Prefer hands/forearms/partial body crops; keep it editorial and premium.",
			"Human elements must not alter the product; product remains the hero and perfectly accurate.",
		} {
			b.WriteString("- " + line + "\n")
		}
		b.WriteString("\n")
	}

	if custom := strings.TrimSpace(opts.Custom); custom != "" {
		b.WriteString("ADDITIONAL NOTES:\n")
		b.WriteString("- " + custom + "\n\n")
	}

	b.WriteString("FRAMES (generate one image per frame):\n")
	for i, fr := range frames {
		b.WriteString(fmt.Sprintf("\nFrame %d: %s\n", i+1, fr.Title))
		b.WriteString("- Template ID: " + fr.ID + "\n")
		b.WriteString("- Concept: " + fr.Concept + "\n")
		b.WriteString("- Execution:\n")
		for _, line := range fr.Execution {
			b.WriteString("  - " + line + "\n")
		}
	}
	b.WriteString("\n")

	b.WriteString("NEGATIVE PROMPT (avoid):\n")
	for _, line := range []string{
		"distorted product", "incorrect logo", "wrong typography", "misspelled label text",
		"product substitution", "different product than reference", "invented branding", "invented brand name",
		"extra text overlays", "watermark", "low resolution", "blurry", "overexposed highlights",
		"dirty/noisy background", "warped perspective", "deformed container", "unreadable branding",
		"cheap stock-photo look", "random readable text (except real product label)",
		"letterbox", "pillarbox", "bars", "black bars", "white bars", "cinematic bars",
		"border", "frame", "white border", "black border", "outline border", "stroke border", "matte border",
		"picture frame", "edge frame", "thin border", "thick border",
		"margin", "padding", "canvas edge", "blank edge", "empty edge", "solid color edge", "white edge", "black edge",
		"vignette border",
	} {
		b.WriteString("- " + line + "\n")
	}
	b.WriteString("\n")

	b.WriteString("OUTPUT RULES:\n")
	b.WriteString(fmt.Sprintf("- Return exactly %d images.\n", out.Count))
	b.WriteString("- Images only. No text, no JSON.\n")

	return strings.TrimSpace(b.String()), out
}

func uniq(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func writeSection(b *strings.Builder, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	b.WriteString("- " + title + ":\n")
	for _, line := range lines {
		b.WriteString("  - " + line + "\n")
	}
}

func cloneFrameTemplate(t FrameTemplate) FrameTemplate {
	t.Execution = append([]string(nil), t.Execution...)
	return t
}

func normalizeAspectRatio(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	a, errA := strconv.Atoi(strings.TrimSpace(parts[0]))
	b, errB := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errA != nil || errB != nil || a <= 0 || b <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", a, b)
}
