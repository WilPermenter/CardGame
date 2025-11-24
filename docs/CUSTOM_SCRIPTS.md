# Card Custom Script System

This document describes how to write custom scripts for card abilities in the `CustomScript` field of card definitions.

## Overview

Scripts are executed when a card is played (ETB - Enter The Battlefield) or cast. Scripts consist of function calls separated by semicolons or newlines.

### Syntax

```
FunctionName(arg1, arg2, arg3)
```

Multiple commands can be chained:
```
FunctionName(arg1, arg2); AnotherFunction(arg1)
```

String arguments should be quoted with single or double quotes:
```
Draw(1, 'main', 'caster')
```

## Target References

Many functions accept a "target" argument. These are the valid target references:

### Player Targets
| Reference | Description |
|-----------|-------------|
| `caster`, `self`, `owner` | The player who played/owns the card |
| `opponent`, `enemy` | The opposing player |
| `target` | The targeted player (if card targets a player) |

### Creature Targets
| Reference | Description |
|-----------|-------------|
| `target` | The targeted creature (set by player selection) |
| `self` | The card itself (for ETB effects on the creature) |
| `<instanceId>` | A specific creature by its instance ID (integer) |

---

## Available Functions

### Draw
Draw cards from a deck.

```
Draw(count, source, player)
```

| Argument | Type | Values |
|----------|------|--------|
| count | integer | Number of cards to draw |
| source | string | `main`/`deck` or `vault`/`land` |
| player | string | Player reference |

**Examples:**
```
Draw(1, 'main', 'caster')      // Caster draws 1 from main deck
Draw(2, 'vault', 'opponent')   // Opponent draws 2 from vault
```

---

### Damage
Deal damage to a player or creature.

```
Damage(amount, target)
```

| Argument | Type | Values |
|----------|------|--------|
| amount | integer | Damage to deal |
| target | string | `opponent`/`enemy` for players, `target` or instanceId for creatures |

**Examples:**
```
Damage(3, 'opponent')   // Deal 3 damage to opponent
Damage(2, 'target')     // Deal 2 damage to target creature
```

**Note:** If this reduces a player to 0 or less life, the game ends.

---

### DamageCreature
Deal damage specifically to a creature (clearer intent than Damage).

```
DamageCreature(amount, target)
```

| Argument | Type | Values |
|----------|------|--------|
| amount | integer | Damage to deal |
| target | string | `target` or creature instanceId |

**Examples:**
```
DamageCreature(3, 'target')   // Deal 3 damage to target creature
```

---

### Heal
Restore life to a player or health to a creature.

```
Heal(amount, target)
```

| Argument | Type | Values |
|----------|------|--------|
| amount | integer | Health/life to restore |
| target | string | Player or creature reference |

**Examples:**
```
Heal(5, 'caster')    // Caster gains 5 life
Heal(2, 'target')    // Target creature gains 2 health (capped at max)
```

**Note:** Creature healing cannot exceed max health.

---

### Buff
Modify a creature's attack and health.

```
Buff(attack, health, target)
```

| Argument | Type | Values |
|----------|------|--------|
| attack | integer | Attack modifier (can be negative) |
| health | integer | Health modifier (can be negative) |
| target | string | `target`, `self`, or creature instanceId |

**Examples:**
```
Buff(2, 2, 'target')    // Target creature gets +2/+2
Buff(3, 0, 'self')      // This creature gets +3/+0
Buff(-1, -1, 'target')  // Target creature gets -1/-1
```

**Note:** Buffs are currently permanent. Health buff also increases current health.

---

### GainMana
Add mana to a player's mana pool.

```
GainMana(color, amount, player)
```

| Argument | Type | Values |
|----------|------|--------|
| color | string | `white`/`w`, `blue`/`u`, `black`/`b`, `red`/`r`, `green`/`g`, `colorless`/`c` |
| amount | integer | Amount of mana to add |
| player | string | Player reference |

**Examples:**
```
GainMana('green', 2, 'caster')     // Caster gains 2 green mana
GainMana('colorless', 1, 'caster') // Caster gains 1 colorless mana
```

---

### Discard
Force a player to discard cards from their hand.

```
Discard(count, player)
```

| Argument | Type | Values |
|----------|------|--------|
| count | integer | Number of cards to discard |
| player | string | Player reference |

**Examples:**
```
Discard(1, 'opponent')   // Opponent discards 1 card
Discard(2, 'caster')     // Caster discards 2 cards
```

**Note:** Currently discards from the end of hand (not random).

---

### Destroy
Destroy a target creature (sets health to 0).

```
Destroy(target)
```

| Argument | Type | Values |
|----------|------|--------|
| target | string | `target` or creature instanceId |

**Examples:**
```
Destroy('target')   // Destroy target creature
```

---

### TapCreature
Tap a target creature.

```
TapCreature(target)
```

| Argument | Type | Values |
|----------|------|--------|
| target | string | `target` or creature instanceId |

**Examples:**
```
TapCreature('target')   // Tap target creature
```

---

### Bounce
Return a creature to its owner's hand.

```
Bounce(target)
```

| Argument | Type | Values |
|----------|------|--------|
| target | string | `target` or creature instanceId |

**Examples:**
```
Bounce('target')   // Return target creature to owner's hand
```

---

## Script Context

When a script executes, it has access to the following context:

| Property | Description |
|----------|-------------|
| Game | The current game state |
| Card | The FieldCard that triggered the script (for creatures) |
| Caster | The Player who played the card |
| CasterUID | The UID of the caster |
| Target | The targeted FieldCard (if targeting a creature) |
| TargetUID | The targeted player's UID (if targeting a player) |

---

## Card Examples

### ETB Effect (Creature)
```json
{
  "ID": 100,
  "Name": "Healing Priest",
  "CardType": "Creature",
  "CardText": "When this enters, gain 3 life.",
  "CustomScript": "Heal(3, 'caster')"
}
```

### Spell with Target
```json
{
  "ID": 101,
  "Name": "Lightning Bolt",
  "CardType": "Instant",
  "CardText": "Deal 3 damage to target creature.",
  "CustomScript": "DamageCreature(3, 'target')"
}
```

### Multiple Effects
```json
{
  "ID": 102,
  "Name": "Chill Wave",
  "CardType": "Instant",
  "CardText": "Tap target creature and deal 1 damage to it.",
  "CustomScript": "TapCreature('target'); DamageCreature(1, 'target')"
}
```

### Self-Buff
```json
{
  "ID": 103,
  "Name": "Raging Berserker",
  "CardType": "Creature",
  "CardText": "When this enters, it gets +2/+0.",
  "CustomScript": "Buff(2, 0, 'self')"
}
```

### Card Draw
```json
{
  "ID": 104,
  "Name": "Sage of Wisdom",
  "CardType": "Creature",
  "CardText": "When this enters, draw a card.",
  "CustomScript": "Draw(1, 'main', 'caster')"
}
```

---

## Events Generated

Scripts generate events that are broadcast to clients:

| Event Type | Description |
|------------|-------------|
| `ScriptDraw` | Cards were drawn |
| `ScriptDamage` | Damage was dealt |
| `ScriptHeal` | Health/life was restored |
| `ScriptBuff` | Creature was buffed |
| `ScriptManaAdded` | Mana was added to pool |
| `ScriptDiscard` | Cards were discarded |
| `ScriptDestroy` | Creature was destroyed |
| `ScriptTap` | Creature was tapped |
| `ScriptBounce` | Creature was bounced |
| `ScriptError` | Script execution error |

---

## Not Yet Implemented

The following features are planned but not yet available:

- **Temporary effects** ("until end of turn", "until end of combat")
- **Triggered abilities** ("whenever", "at the beginning of")
- **Conditional logic** (if/else)
- **Targeting restrictions** (only opponent's creatures, only untapped, etc.)
- **Area effects** (all creatures, all opponent's creatures)
- **Counter manipulation** (+1/+1 counters, etc.)
