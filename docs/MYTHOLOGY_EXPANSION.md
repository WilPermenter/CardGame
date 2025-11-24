# Mythology Expansion - Design Document

## Overview
A massive expansion bringing gods and mythological creatures from various pantheons into the card game. Each pantheon aligns with MTG-style color identities and introduces unique mechanics inspired by Hearthstone, Riftbound, Elder Scrolls Legends, and Eternal.

## Color-Pantheon Alignment

| Color | Primary Themes | Primary Pantheons | Secondary Pantheons |
|-------|---------------|-------------------|---------------------|
| **White** | Order, Justice, Light, Protection | Greek (Olympus), Egyptian (Solar) | Norse (Aesir), Hindu (Devas) |
| **Blue** | Knowledge, Trickery, Water, Mind | Norse (Loki), Greek (Sea), Egyptian (Thoth) | Japanese (Moon), Mayan (Feathered Serpent) |
| **Black** | Death, Darkness, Power, Sacrifice | Egyptian (Underworld), Greek (Underworld), Norse (Hel) | Celtic (Morrigan), Mayan (Death) |
| **Red** | War, Chaos, Fire, Passion | Norse (Thor), Greek (War), Hindu (Destruction) | Japanese (Storm), Chinese (Warriors) |
| **Green** | Nature, Growth, Life, Beasts | Celtic (Nature), Greek (Wild), Norse (Vanir) | Japanese (Nature), Hindu (Nature) |

## New Mechanics

### 1. Devotion (from MTG Theros)
**Format**: `Devotion(color, effect_per_devotion)`
- Count permanents of a color on your field
- Scale effects based on devotion count
- Encourages mono-color or heavy splash decks

### 2. Divine Shield (from Hearthstone)
**Format**: `DivineShield` (ability)
- Prevents the first instance of damage
- Removed after blocking damage once
- Great for aggressive creatures

### 3. Prophecy (from Elder Scrolls Legends)
**Format**: `Prophecy(effect)` (trigger)
- Activates when drawn from damage (opponent hits your face)
- Can be cast for free immediately
- Creates comeback mechanics

### 4. Invoke (from Hearthstone)
**Format**: `Invoke(god_id, effect)`
- Bonus effect if you control your God/Leader
- Encourages playing your leader early
- Synergy with specific god cards

### 5. Deathrattle / Last Breath
**Format**: `OnDeath(effect)`
- Triggers when this creature dies
- Already have OnCreatureDeath, this is self-referential

### 6. Wrath (original - inspired by Riftbound)
**Format**: `OnWrath(threshold, effect)` (trigger)
- Triggers when you take X or more damage in a turn
- Comeback mechanic for control decks
- Rewards taking hits strategically

### 7. Ascend (from MTG Ixalan)
**Format**: `Ascend(effect)`
- Permanent buff when you control 10+ cards (field + lands)
- "City's Blessing" style permanent upgrade

### 8. Rally (from Eternal)
**Format**: `OnRally(min_attackers, effect)`
- Triggers when attacking with X or more creatures
- Rewards going wide strategies

### 9. Corrupt (original)
**Format**: `Corrupt(target, effect)`
- Debuff enemy creatures permanently
- Opposite of Buff - reduces stats
- Can stack

### 10. Bless (original)
**Format**: `Bless(target, turns, effect)`
- Temporary buff that lasts X turns
- Auto-removes after duration
- Good for combat tricks

### 11. Smite (original - thematic!)
**Format**: `Smite(damage, condition)`
- Deal damage to creatures meeting a condition
- e.g., "Smite 3 damage to all tapped creatures"

### 12. Resurrection
**Format**: `Resurrect(count, condition)`
- Return creatures from graveyard to field
- Can have conditions (random, specific type, etc.)

### 13. Transform (from Hearthstone)
**Format**: `Transform(target, new_card_id)`
- Replace a creature with a different one
- Bypasses death triggers

### 14. Discover (from Hearthstone - simplified)
**Format**: `Discover(card_type, count)`
- Add random cards of type to hand
- Simplified version - just draws from pool

## Pantheon Breakdown

### Norse Pantheon
**Gods**: Odin, Thor, Loki, Freya, Hel, Tyr, Heimdall, Baldur
**Creatures**: Valkyries, Einherjar, Wolves (Fenrir, Geri, Freki), Ravens (Huginn, Muninn), Frost Giants, Fire Giants, Dwarves, Elves
**Themes**:
- Odin (Blue/Black): Knowledge, sacrifice, ravens
- Thor (Red): Lightning, combat, hammers
- Loki (Blue): Tricks, transformation, chaos
- Freya (White/Green): Valkyries, love, fertility
- Hel (Black): Death, undead, cold

### Greek Pantheon
**Gods**: Zeus, Poseidon, Hades, Athena, Ares, Apollo, Artemis, Hermes, Hephaestus, Aphrodite
**Creatures**: Titans, Cyclopes, Minotaurs, Centaurs, Gorgons, Hydra, Cerberus, Pegasus, Sirens
**Themes**:
- Zeus (White/Blue): Lightning, sky, authority
- Poseidon (Blue): Sea, earthquakes, horses
- Hades (Black): Underworld, wealth, souls
- Athena (White): Wisdom, strategy, crafts
- Ares (Red): War, bloodshed, violence

### Egyptian Pantheon
**Gods**: Ra, Anubis, Set, Horus, Thoth, Bastet, Sobek, Osiris, Isis, Sekhmet
**Creatures**: Mummies, Sphinxes, Scarabs, Jackals, Serpents, Crocodiles
**Themes**:
- Ra (White): Sun, creation, life
- Anubis (Black): Death, mummification, judgment
- Set (Black/Red): Chaos, storms, desert
- Thoth (Blue): Knowledge, magic, moon
- Horus (White/Blue): Sky, kingship, protection

### Celtic Pantheon
**Gods**: Cernunnos, Morrigan, Brigid, Lugh, Dagda, Nuada
**Creatures**: Druids, Fae, Banshees, Cu Sith, Selkies, Green Men
**Themes**:
- Cernunnos (Green): Beasts, forest, hunting
- Morrigan (Black): Death, war, fate
- Brigid (White/Red): Healing, fire, crafts
- Dagda (Green): Earth, abundance, magic

### Hindu Pantheon
**Gods**: Shiva, Kali, Ganesh, Vishnu, Brahma, Indra, Agni, Hanuman
**Creatures**: Asuras, Devas, Nagas, Garudas, Rakshasas
**Themes**:
- Shiva (Black/Blue): Destruction, transformation, dance
- Kali (Black/Red): Death, time, destruction
- Ganesh (White): Wisdom, beginnings, obstacles
- Agni (Red): Fire, sacrifice, messenger

### Japanese Pantheon
**Gods**: Amaterasu, Susanoo, Tsukuyomi, Raijin, Fujin, Inari
**Creatures**: Oni, Kitsune, Tengu, Yokai, Kami, Dragons
**Themes**:
- Amaterasu (White): Sun, order, purity
- Susanoo (Red/Blue): Storms, seas, chaos
- Tsukuyomi (Blue/Black): Moon, night, time
- Raijin (Red): Thunder, lightning, drums

### Mayan Pantheon
**Gods**: Kukulkan, Ah Puch, Chaac, Ixchel, Hun Hunahpu
**Creatures**: Jaguars, Serpents, Bats, Spirits
**Themes**:
- Kukulkan (Blue/Green): Wind, wisdom, feathered serpent
- Ah Puch (Black): Death, decay, underworld
- Chaac (Blue/Red): Rain, storms, agriculture

### Chinese Pantheon
**Gods/Heroes**: Sun Wukong, Nezha, Guan Yu, Chang'e, Erlang Shen
**Creatures**: Dragons, Phoenix, Qilin, Foo Dogs
**Themes**:
- Sun Wukong (Red/Green): Trickery, strength, rebellion
- Guan Yu (Red/White): War, honor, loyalty
- Chang'e (Blue/White): Moon, immortality

## Deck Archetypes by Color

### White Archetypes
1. **Olympian Justice** (Zeus) - Control, board wipes, divine authority
2. **Solar Dynasty** (Ra) - Life gain, tokens, light-based removal
3. **Valkyrie's Chosen** (Freya) - Aggro, flying, death triggers
4. **Wisdom's Path** (Athena/Ganesh) - Midrange, card draw, protection

### Blue Archetypes
1. **Trickster's Gambit** (Loki) - Mill, transformation, chaos
2. **Ocean's Wrath** (Poseidon) - Sea creatures, bounce, control
3. **Lunar Mysteries** (Thoth/Tsukuyomi) - Card draw, prediction, combo
4. **Feathered Serpent** (Kukulkan) - Wind, tempo, evasion

### Black Archetypes
1. **Underworld Sovereign** (Hades/Anubis) - Resurrection, sacrifice, control
2. **Realm of Hel** (Hel) - Undead, cold, inevitability
3. **Chaos Incarnate** (Set) - Destruction, aggro, burn
4. **Phantom Queen** (Morrigan) - Death triggers, war, fate

### Red Archetypes
1. **Thunder God's Fury** (Thor/Raijin) - Burn, aggro, direct damage
2. **Flames of Destruction** (Kali/Agni) - Sacrifice, burn, glass cannon
3. **Monkey King's Rebellion** (Sun Wukong) - Combat tricks, clone, chaos
4. **War Eternal** (Ares/Guan Yu) - Aggro, combat buffs, rally

### Green Archetypes
1. **Wild Hunt** (Cernunnos/Artemis) - Beasts, ramp, big creatures
2. **Nature's Wrath** (Gaia concept) - Ramp, stompy, overwhelming force
3. **Spirit of the Forest** (Celtic Fae) - Tokens, enchantments, life
4. **Monkey King's Journey** (Sun Wukong) - Ramp, combat, transformation

### Multi-Color Archetypes
1. **Chaos Twins** (Loki + Hel, Blue/Black) - Mill + Resurrection combo
2. **Storm Lords** (Thor + Poseidon, Red/Blue) - Burn + Bounce tempo
3. **Death's Harvest** (Anubis + Osiris, Black/White) - Reanimator
4. **Primal Rage** (Shiva + Kali, Black/Red) - Destruction aggro

## Card ID Allocation

| Range | Pantheon/Category |
|-------|-------------------|
| 300-399 | Norse Pantheon |
| 400-499 | Greek Pantheon |
| 500-599 | Egyptian Pantheon |
| 600-699 | Celtic Pantheon |
| 700-799 | Hindu Pantheon |
| 800-899 | Japanese Pantheon |
| 900-999 | Mayan/Chinese Pantheon |
| 1000-1099 | Multi-color/Neutral |
| 1100-1199 | Tokens |

## Deck ID Allocation

| Range | Category |
|-------|----------|
| 300-319 | Norse Decks |
| 320-339 | Greek Decks |
| 340-359 | Egyptian Decks |
| 360-379 | Celtic Decks |
| 380-399 | Hindu Decks |
| 400-419 | Japanese Decks |
| 420-439 | Mayan/Chinese Decks |
| 440-459 | Multi-color Decks |

## Implementation Status

### Completed Pantheons

| Pantheon | Cards | Decks | Leaders | Status |
|----------|-------|-------|---------|--------|
| Norse | 300-385 | 300-304 | 5 (Odin, Thor, Loki, Freya, Hel) | COMPLETE |
| Greek | 400-471 | 320-324 | 5 (Zeus, Poseidon, Hades, Athena, Ares) | COMPLETE |
| Egyptian | 500-593 | 340-344 | 5 (Ra, Anubis, Set, Thoth, Horus) | COMPLETE |
| Celtic | 600-693 | 360-364 | 5 (Dagda, Morrigan, Lugh, Brigid, Cernunnos) | COMPLETE |
| Hindu | 700-791 | 380-384 | 5 (Shiva, Vishnu, Brahma, Ganesha, Kali) | COMPLETE |
| Japanese | 800-892 | 400-404 | 5 (Amaterasu, Susanoo, Tsukuyomi, Izanagi, Raijin) | COMPLETE |
| Chinese | 900-992 | 420-424 | 5 (Jade Emperor, Sun Wukong, Nezha, Guanyin, Dragon King) | COMPLETE |

### Summary Statistics
- **Total New Cards**: ~400+ cards across 7 pantheons
- **Total New Decks**: 35 themed decks (5 per pantheon)
- **Total New Leaders**: 35 god leaders
- **New Mechanics Added**: 18+ scripting functions

### Documentation
See individual pantheon files in `/docs/pantheons/`:
- [Norse](pantheons/NORSE.md)
- [Greek](pantheons/GREEK.md)
- [Egyptian](pantheons/EGYPTIAN.md)
- [Celtic](pantheons/CELTIC.md)
- [Hindu](pantheons/HINDU.md)
- [Japanese](pantheons/JAPANESE.md)
- [Chinese](pantheons/CHINESE.md)

## Notes
- Each god should feel unique and powerful as a Leader
- Support cards should synergize with their god's themes
- Cross-pantheon synergies for multi-color decks
- Keep mana costs balanced for the game's tempo
- Tokens should be simple but flavorful
