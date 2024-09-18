package main

import (
	"strings"
	"sync"
)

// сюда писать код
// на сервер грузить только этот файл

type Player struct {
	name     string
	site     Site
	backpack []string
	out      chan string
}

func NewPlayer(name string) *Player {
	return &Player{
		name: name,
		out:  make(chan string),
	}
}

func (p *Player) GetOutput() <-chan string {
	return p.out
}

func (p *Player) HandleInput(s string) {

	game.mu.Lock() // xxx global game lock
	defer game.mu.Unlock()

	args := strings.Split(s, " ")

	switch args[0] {
	case "осмотреться":
		p.out <- p.look()
	case "идти":
		p.out <- p.goTo(args[1])
	case "одеть":
		p.out <- p.dress(args[1])
	case "взять":
		p.out <- p.take(args[1])
	case "применить":
		p.out <- p.apply(args[1], args[2])
	case "сказать":
		p.tell(strings.Join(args[1:], " "))
	case "сказать_игроку":
		p.tellPlayer(args[1], strings.Join(args[2:], " "))
	default:
		p.out <- "неизвестная команда"
	}
}

func (p *Player) look() string {
	return p.site.LookAround(p)
}

func (p *Player) goTo(siteName string) string {

	site, msg := p.site.GoTo(siteName)
	if site != nil {
		p.site = site
	}

	return msg
}

func (p *Player) dress(item string) string {

	// одеть можно только рюкзак

	if item != "рюкзак" {
		return "это невозможно одеть"
	}

	if !p.site.PopItem(item) {
		return "нет такого"
	}

	p.backpack = []string{}
	return "вы одели: рюкзак"
}

func (p *Player) take(item string) string {

	if p.backpack == nil {
		return "некуда класть"
	}

	if !p.site.PopItem(item) {
		return "нет такого"
	}

	p.backpack = append(p.backpack, item)
	return "предмет добавлен в инвентарь: " + item
}

func (p *Player) apply(item, toObj string) string {
	for _, it := range p.backpack {
		if it == item {
			_, msg := p.site.ApplyItemTo(item, toObj)
			return msg
		}
	}
	return "нет предмета в инвентаре - " + item
}

func (p *Player) tell(msg string) {

	if msg == "" {
		msg = p.name + " выразительно молчит"
	} else {
		msg = p.name + " говорит: " + msg
	}

	siteName := p.site.Name()
	for _, peer := range game.players {
		if peer.site.Name() == siteName {
			peer.out <- msg
		}
	}
}

func (p *Player) tellPlayer(player string, msg string) {

	peer := game.players[player]
	if peer == nil || peer.site.Name() != p.site.Name() {
		p.out <- "тут нет такого игрока"
		return
	}

	if msg == "" {
		peer.out <- p.name + " выразительно молчит, смотря на вас"
	} else {
		peer.out <- p.name + " говорит вам: " + msg
	}
}

type Site interface {
	Name() string
	ComeTo() string
	GoTo(site string) (Site, string)
	LookAround(*Player) string
	PopItem(item string) bool
	ApplyItemTo(item, toObj string) (bool, string)
}

type Way interface {
	Dest() Site
	Go() (Site, string)
}

type Object interface {
	Name() string
	Apply(item string) (bool, string)
}

type site struct {
	name        string
	description string
	ways        []Way
	items       map[string][]string
	objects     []Object
}

func (s *site) Name() string {
	return s.name
}

func (s *site) ComeTo() string {
	return strings.Join([]string{s.description, s.canGoto()}, ". ")
}

func (s *site) GoTo(siteName string) (Site, string) {
	for _, way := range s.ways {
		if way.Dest().Name() == siteName {
			return way.Go()
		}
	}
	return nil, "нет пути в " + siteName
}

func (s *site) LookAround(p *Player) string {
	sents := make([]string, 0, 3)
	sents = append(sents, s.description)

	if se := s.canGoto(); se != "" {
		sents = append(sents, se)
	}

	if se := s.otherPlayers(p); se != "" {
		sents = append(sents, se)
	}

	return strings.Join(sents, ". ")
}

func (s *site) canGoto() string {
	names := make([]string, len(s.ways))

	for i, way := range s.ways {
		names[i] = way.Dest().Name()
	}

	if len(names) == 0 {
		return ""
	}

	return "можно пройти - " + strings.Join(names, ", ")
}

func (s *site) otherPlayers(p *Player) string {
	var names []string

	for _, op := range game.players {
		if op.site.Name() == s.name && op.name != p.name {
			names = append(names, op.name)
		}
	}

	if len(names) == 0 {
		return ""
	}

	return "Кроме вас тут ещё " + strings.Join(names, ", ")
}

func (s *site) PopItem(itemName string) bool {

	for k, vs := range s.items {
		for j, v := range vs {
			if v == itemName {
				vs[j] = vs[len(vs)-1]
				vs[len(vs)-1] = ""
				s.items[k] = vs[:len(vs)-1]
				return true
			}
		}
	}

	return false
}

func (s *site) ApplyItemTo(itemName, toObjName string) (bool, string) {

	for _, obj := range s.objects {
		if obj.Name() == toObjName {
			return obj.Apply(itemName)
		}
	}

	return false, "не к чему применить"
}

var _ Site = (*site)(nil)

type way struct {
	dest Site
}

func (w way) Dest() Site {
	return w.dest
}

func (w way) Go() (Site, string) {
	return w.dest, w.dest.ComeTo()
}

type Door struct {
	way
	name   string
	isOpen bool
}

func (d *Door) Name() string {
	return d.name
}

func (d *Door) Go() (Site, string) {
	if !d.isOpen {
		return nil, "дверь закрыта"
	}
	return d.way.Go()
}

func (d *Door) Apply(itemName string) (bool, string) {

	if itemName != "ключи" {
		return false, "нельзя применить " + itemName + " к двери"
	}

	d.isOpen = !d.isOpen

	if d.isOpen {
		return true, "дверь открыта"
	} else {
		return true, "дверь закрыта"
	}
}

type Room struct {
	site
}

func (s *Room) LookAround(_ *Player) string {
	var sb strings.Builder

	onTable := s.items["на столе"]
	onChair := s.items["на стуле"]

	if len(onTable) == 0 && len(onChair) == 0 {
		sb.WriteString("пустая комната")
	} else {
		if sb.Len() != 0 {
			sb.WriteString(". ")
		}
		n := 0
		if len(onTable) > 0 {
			n += len(onTable)
			sb.WriteString("на столе: ")
			sb.WriteString(strings.Join(onTable, ", "))
		}
		if len(onChair) > 0 {
			if n > 0 {
				sb.WriteString(", ")
			}
			n += len(onTable)
			sb.WriteString("на стуле - ")
			sb.WriteString(strings.Join(onChair, ", "))
		}
	}

	sb.WriteString(". ")
	sb.WriteString(s.canGoto())

	return sb.String()
}

type Kitchen struct {
	site
}

func (s *Kitchen) LookAround(p *Player) string {
	var sb strings.Builder
	sb.WriteString("ты находишься на кухне, на столе чай, надо ")

	if len(p.backpack) < 2 { // ключи, конспекты
		sb.WriteString("собрать рюкзак и ")
	}

	sb.WriteString("идти в универ. ")
	sb.WriteString(s.canGoto())

	if op := s.otherPlayers(p); op != "" {
		sb.WriteString(". ")
		sb.WriteString(op)
	}

	return sb.String()
}

type Hallway struct {
	site
}

type Outsite struct {
	site
}

var game = struct {
	mu      sync.Mutex
	kitchen *Kitchen
	room    *Room
	hallway *Hallway
	outsite *Outsite
	players map[string]*Player
}{}

func initGame() {
	game.mu.Lock()
	defer game.mu.Unlock()

	game.players = map[string]*Player{}

	kitchen := &Kitchen{site{
		name:        "кухня",
		description: "кухня, ничего интересного",
	}}

	room := &Room{site{
		name:        "комната",
		description: "ты в своей комнате",
		items: map[string][]string{
			"на столе": {"ключи", "конспекты"},
			"на стуле": {"рюкзак"},
		},
	}}

	outDoor := &Door{
		name: "дверь",
	}

	hallway := &Hallway{site{
		name:        "коридор",
		description: "ничего интересного",
		objects: []Object{
			outDoor,
		},
	}}

	outsite := &Outsite{site{
		name:        "улица",
		description: "на улице весна",
	}}

	outDoor.dest = outsite

	hallway.ways = []Way{
		&way{kitchen},
		&way{room},
		outDoor,
	}

	kitchen.ways = []Way{
		way{hallway},
	}

	room.ways = []Way{
		way{hallway},
	}

	outsite.ways = []Way{
		way{&site{name: "домой"}}, // xxx stub
	}

	game.kitchen = kitchen
	game.hallway = hallway
	game.room = room
	game.outsite = outsite
}

func addPlayer(p *Player) {
	game.mu.Lock()
	p.site = game.kitchen
	game.players[p.name] = p
	game.mu.Unlock()
}
