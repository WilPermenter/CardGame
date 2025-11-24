# New Mechanics Reference

## Devotion
**Source**: MTG Theros block
**Format**: `Devotion(color, effect_per_pip)`

Counts the number of permanents of a specific color on your field and scales an effect.

**Example**:
- `Devotion(white, Damage(1, opponent))` - Deal 1 damage per white permanent
- `Devotion(blue, Draw(1, main, caster))` - Draw 1 card per blue permanent

**Implementation Notes**:
- Counts creatures, artifacts, and leaders on field
- Color is determined by the mana cost of cards
- Colorless cards don't count toward devotion

---

## Divine Shield
**Source**: Hearthstone
**Format**: `DivineShield` (Ability keyword)

Creature ability that prevents the first instance of damage taken.

**Behavior**:
- First damage dealt to creature is negated entirely
- Shield is then removed
- Visual indicator shows shield active
- Can be re-applied by effects

**Implementation Notes**:
- Stored as `Status["DivineShield"] = 1`
- Checked in combat resolution before applying damage
- Removed after blocking damage

---

## Prophecy
**Source**: Elder Scrolls Legends
**Format**: `Prophecy(effect)` (Trigger)

A trigger that activates when the card is drawn due to taking damage.

**Behavior**:
- Only triggers when drawn from "rune breaks" (damage to face)
- Effect executes immediately for free
- Creates comeback potential

**Implementation Notes**:
- Requires tracking "damage draw" vs "normal draw"
- New draw source type: "prophecy"
- OnProphecy trigger type

---

## Invoke
**Source**: Hearthstone Descent of Dragons
**Format**: `Invoke(effect)`

Bonus effect that triggers if you control your Leader/God on the field.

**Example**:
- `Invoke(Buff(2, 2, friendly_creatures))` - If leader on field, buff all allies

**Implementation Notes**:
- Check if player.Leader == 0 AND has a Leader-type card on field
- Simple boolean check before executing effect

---

## OnDeath (Deathrattle/Last Breath)
**Source**: Hearthstone/LoR
**Format**: `OnDeath(condition, effect)` (Self-referential)

Triggers when THIS creature dies, not any creature.

**Example**:
- `OnDeath(any, Summon(1100, 2))` - Summon 2 tokens when this dies

**Implementation Notes**:
- Different from OnCreatureDeath which watches all deaths
- Self-referential trigger
- Stored on the creature, fires on its death

---

## Wrath
**Source**: Original (Riftbound inspired)
**Format**: `OnWrath(threshold, effect)` (Trigger)

Artifact/creature trigger that fires when you take X+ damage in a single instance.

**Example**:
- `OnWrath(5, Draw(2, main, caster))` - Draw 2 when you take 5+ damage

**Implementation Notes**:
- Threshold checked per damage instance
- Encourages "taking hits" strategically
- Synergizes with life gain

---

## Ascend
**Source**: MTG Ixalan
**Format**: `Ascend(effect)`

Checks if you have 10+ permanents (field + lands) and grants a permanent buff.

**Example**:
- `Ascend(Buff(3, 3, self))` - Gain +3/+3 if you have 10+ permanents

**Implementation Notes**:
- One-time check (usually on enter or trigger)
- "City's Blessing" style - once earned, keeps effect
- Count includes creatures, artifacts, lands

---

## Rally
**Source**: Eternal Card Game
**Format**: `OnRally(min_attackers, effect)` (Trigger)

Triggers when you attack with X or more creatures simultaneously.

**Example**:
- `OnRally(3, BuffAll(1, 1, friendly_creatures))` - All allies get +1/+1 when attacking with 3+

**Implementation Notes**:
- Checked in declare_attacks phase
- Counts attacking creatures
- Good for "go wide" strategies

---

## Corrupt
**Source**: Original
**Format**: `Corrupt(attack_mod, health_mod, target)`

Permanent debuff to enemy creatures (opposite of Buff).

**Example**:
- `Corrupt(-2, -2, target:enemy)` - Target enemy gets -2/-2

**Implementation Notes**:
- Uses negative modifiers
- Can kill creatures if health drops to 0
- Stacks with other corruptions

---

## Bless
**Source**: Original
**Format**: `Bless(attack_mod, health_mod, turns, target)`

Temporary buff that expires after X turns.

**Example**:
- `Bless(3, 3, 2, target:friendly)` - Target ally gets +3/+3 for 2 turns

**Implementation Notes**:
- Track turn counter on creature
- Remove buff when counter reaches 0
- Counter decrements at end of owner's turn

---

## Smite
**Source**: Original (thematic!)
**Format**: `Smite(damage, condition)`

Divine damage that hits creatures meeting a condition.

**Example**:
- `Smite(3, tapped)` - Deal 3 to all tapped creatures
- `Smite(5, enemy_with_attack_over_5)` - Deal 5 to enemies with 5+ attack

**Implementation Notes**:
- Condition parser needed
- Conditions: tapped, untapped, damaged, attacking, defending

---

## Resurrect
**Source**: Various
**Format**: `Resurrect(count, condition, player)`

Return creatures from graveyard to battlefield.

**Example**:
- `Resurrect(1, any, caster)` - Resurrect 1 random creature
- `Resurrect(2, creature, caster)` - Resurrect 2 creatures

**Implementation Notes**:
- Similar to Reanimate but multiple targets
- Can have conditions on what to resurrect
- "any" means random

---

## Transform
**Source**: Hearthstone
**Format**: `Transform(target, new_card_id)`

Replace a creature with a completely different one.

**Example**:
- `Transform(target:any, 1101)` - Transform target into a 1/1 Sheep

**Implementation Notes**:
- Removes original creature (NO death trigger)
- Creates new creature in its place
- New creature has summoning sickness

---

## Discover
**Source**: Hearthstone (simplified)
**Format**: `Discover(card_type, count)`

Add random cards to your hand from a pool.

**Example**:
- `Discover(creature, 2)` - Add 2 random creatures to hand
- `Discover(spell, 1)` - Add 1 random spell to hand

**Implementation Notes**:
- Simplified version without choice UI
- Pulls from CardDB based on type
- Filtered by card type

---

## Storm (new!)
**Source**: MTG
**Format**: `Storm(effect)`

Copy this spell for each spell cast this turn.

**Implementation Notes**:
- Track spells cast this turn per player
- Create copies on cast
- Very powerful - use carefully

---

## Lifesteal
**Source**: Hearthstone/MTG (Lifelink)
**Format**: `Lifesteal` (Ability keyword)

Damage dealt by this creature heals your leader.

**Implementation Notes**:
- Triggers on combat damage
- Heal amount equals damage dealt
- Stored as ability keyword

---

## Freeze
**Source**: Hearthstone
**Format**: `Freeze(target)`

Prevent a creature from attacking next turn.

**Implementation Notes**:
- Sets Status["Frozen"] = 1
- Frozen creatures can't attack
- Unfreezes at start of controller's turn
