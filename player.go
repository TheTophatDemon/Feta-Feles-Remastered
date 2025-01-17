/*
Copyright (C) 2021 Alexander Lunsford

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/thetophatdemon/feta-feles-rebirth/audio"
	"github.com/thetophatdemon/feta-feles-rebirth/vmath"
)

const (
	PL_SHOOT_FREQ = 0.2
	PL_WARP_THRESHOLD = 1.0
)

type Player struct {
	*Actor
	hurt, ascended   bool
	shootTimer       float64
	hurtTimer        float64
	lastShootDir     *vmath.Vec2f
	warpCooldown float64
}

var plSpriteNormal *Sprite
var plSpriteShoot *Sprite
var plSpriteHurt *Sprite
var plSpriteAscended *Sprite

func init() {
	plSpriteNormal = NewSprite(image.Rect(0, 0, 16, 16), vmath.NewVec(-8.0, -8.0), false, false, 0)
	plSpriteShoot = NewSprite(image.Rect(16, 0, 32, 16), vmath.NewVec(-8.0, -8.0), false, false, 0)
	plSpriteHurt = NewSprite(image.Rect(32, 0, 32+16, 16), vmath.NewVec(-8.0, -8.0), false, false, 0)
	plSpriteAscended = NewSprite(image.Rect(48, 0, 64, 16), vmath.NewVec(-8.0, -8.0), false, false, 0)
}

func AddPlayer(game *Game, x, y float64) *Object {
	player := &Player{
		Actor:        NewActor(120.0, 500_000.0, 50_000.0),
		hurt:         false,
		ascended:     false,
		lastShootDir: vmath.NewVec(1.0, 0.0),
	}
	//player.Actor.ignoreBounds = true
	obj := &Object{
		pos: vmath.NewVec(x, y), radius: 6.0, colType: CT_PLAYER,
		sprites: []*Sprite{
			plSpriteNormal,
		},
		components:   []Component{player},
		drawPriority: 10,
	}
	game.AddObject(obj)

	return obj
}

func (player *Player) Update(game *Game, obj *Object) {
	//Attack
	if player.shootTimer <= 0.0 {
		//Set direction
		var dir *vmath.Vec2f
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) { //Shoot in direction of mouse click
			cx, cy := ebiten.CursorPosition()
			rPos := obj.pos.Clone().Sub(game.camPos).Add(vmath.NewVec(SCR_WIDTH_H, SCR_HEIGHT_H))
			dir = (vmath.NewVec(float64(cx), float64(cy))).Sub(rPos)
			player.lastShootDir = dir
		} else if ebiten.IsKeyPressed(ebiten.KeySpace) { //Or shoot in direction of last movement
			if player.lastShootDir == nil {
				player.lastShootDir = player.facing.Clone()
			}
			dir = player.lastShootDir
		} else {
			player.lastShootDir = nil
		}
		//Add shot
		if dir != nil {
			if player.ascended {
				AddBouncyShot(game, obj.pos, dir, 240.0, false, 2)
				player.shootTimer = PL_SHOOT_FREQ / 2.0
			} else {
				AddShot(game, obj.pos, dir, 240.0, false)
				player.shootTimer = PL_SHOOT_FREQ
			}
			audio.PlaySound("player_shot")
			Emit_Signal(SIGNAL_PLAYER_SHOT, obj, nil)
		}
	} else {
		player.shootTimer -= game.deltaTime
	}

	if player.hurtTimer > 0.0 {
		player.hurtTimer -= game.deltaTime
		if int(player.hurtTimer/0.125)%2 == 0 {
			obj.hidden = false
		} else {
			obj.hidden = true
		}
		if player.hurtTimer <= 0.0 {
			player.hurt = false
			obj.hidden = false
		}
	}

	//Set sprite
	if player.hurt {
		obj.sprites[0] = plSpriteHurt
	} else {
		if player.ascended {
			obj.sprites[0] = plSpriteAscended
		} else {
			if player.shootTimer > 0.0 {
				obj.sprites[0] = plSpriteShoot
			} else {
				obj.sprites[0] = plSpriteNormal
			}
		}
	}

	//Movement
	var dx, dy float64
	if ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		dy = -1.0
	} else if ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		dy = 1.0
	}

	if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		dx = 1.0
	} else if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		dx = -1.0
	}

	if dx != 0.0 || dy != 0.0 {
		Emit_Signal(SIGNAL_PLAYER_MOVED, obj, nil)
	}

	//Handle boundaries & screen wrapping
	leftWarp := (obj.pos.X <= obj.radius + 4 && dx < 0)
	rightWarp := (obj.pos.X >= game.level.pixelWidth - obj.radius - 4 && dx > 0)
	upWarp := (obj.pos.Y <= obj.radius + 4 && dy < 0)
	downWarp := (obj.pos.Y >= game.level.pixelHeight - obj.radius - 4 && dy > 0)
	if leftWarp || rightWarp || upWarp || downWarp {
		player.warpCooldown += game.deltaTime
		if player.warpCooldown > PL_WARP_THRESHOLD {
			//Warp after pushing against boundary for some time
			switch {
			case leftWarp:
				if hit, _, _ := game.level.SphereIntersects(vmath.NewVec(game.level.pixelWidth-obj.radius - 1, obj.pos.Y), obj.radius); !hit {
					obj.pos.X = game.level.pixelWidth - obj.radius
					player.warpCooldown = 0.0
				}
			case rightWarp:
				if hit, _, _ := game.level.SphereIntersects(vmath.NewVec(obj.radius + 1, obj.pos.Y), obj.radius); !hit {
					obj.pos.X = obj.radius
					player.warpCooldown = 0.0
				}
			case upWarp:
				if hit, _, _ := game.level.SphereIntersects(vmath.NewVec(obj.pos.X, game.level.pixelHeight-obj.radius-1), obj.radius); !hit {
					obj.pos.Y = game.level.pixelHeight - obj.radius
					player.warpCooldown = 0.0
				}
			case downWarp:
				if hit, _, _ := game.level.SphereIntersects(vmath.NewVec(obj.pos.X, obj.radius+1), obj.radius); !hit {
					obj.pos.Y = obj.radius
					player.warpCooldown = 0.0
				}
			}
			if player.warpCooldown == 0.0 {
				player.hurtTimer = 1.0 //Add invincibility frames after warping in case there's an unseen enemy
			}
		}
	} else {
		player.warpCooldown = 0.0
	}

	player.Actor.Move(dx, dy)
	player.Actor.Update(game, obj)
}

func (player *Player) OnCollision(game *Game, obj, other *Object) {
	switch {
	case other.HasColType(CT_ITEM):
		ascend := game.IncLoveCounter(1)
		if ascend {
			if !player.ascended {
				Emit_Signal(SIGNAL_PLAYER_ASCEND, obj, nil)
			}
			player.ascended = true
		}
	case other.HasColType(CT_ENEMY | CT_ENEMYSHOT | CT_EXPLOSION):
		if !player.hurt && player.hurtTimer <= 0.0 {
			player.hurt = true
			player.hurtTimer = 1.0
			lost := false
			if other.colType == CT_EXPLOSION {
				lost = game.DecLoveCounter(20)
			} else {
				lost = game.DecLoveCounter(10)
			}
			if lost && player.ascended {
				player.ascended = false
				audio.PlaySound("descend")
			} else {
				audio.PlaySound("player_hurt")
			}
		}
	}
}
