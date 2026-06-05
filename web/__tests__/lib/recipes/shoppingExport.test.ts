import {
  formatForClipboard,
  formatForAppleNotes,
  formatAsTxt,
  groupByStore,
  toExportGroups,
  formatGroupedForClipboard,
  formatGroupedForAppleNotes,
  formatGroupedAsTxt,
  prepareForExport,
  formatOrigins,
  type Category,
  type ShoppingItem,
  type ItemOrigin
} from '@/lib/recipes/shoppingExport'

const customItems: ShoppingItem[] = [{ id: 'c1', amount: '0.5', unit: 'tsp', name: 'salt' }]

const mealItems: ShoppingItem[] = [
  { amount: '2', unit: 'cups', name: 'flour' },
  { amount: '1', unit: 'tbsp', name: 'sugar' },
  { amount: '100', unit: 'g', name: 'butter' }
]

describe('shoppingExport', () => {
  describe('unit upgrades', () => {
    it('converts 1000g to 1kg', () => {
      const result = formatForClipboard([{ amount: '1000', unit: 'g', name: 'flour' }])
      expect(result).toBe('1 kg - flour')
    })

    it('converts 1500g to 1.5kg', () => {
      const result = formatForClipboard([{ amount: '1500', unit: 'g', name: 'flour' }])
      expect(result).toBe('1.5 kg - flour')
    })

    it('does not convert 999g', () => {
      const result = formatForClipboard([{ amount: '999', unit: 'g', name: 'flour' }])
      expect(result).toBe('999 g - flour')
    })

    it('converts 1000ml to 1L', () => {
      const result = formatForClipboard([{ amount: '1000', unit: 'ml', name: 'water' }])
      expect(result).toBe('1 L - water')
    })

    it('converts 1000mg to 1g', () => {
      const result = formatForClipboard([{ amount: '1000', unit: 'mg', name: 'spice' }])
      expect(result).toBe('1 g - spice')
    })

    it('does not convert unknown units', () => {
      const result = formatForClipboard([{ amount: '1000', unit: 'tsp', name: 'salt' }])
      expect(result).toBe('1000 tsp - salt')
    })

    it('upgrades combined total after summing (600g + 600g = 1.2kg)', () => {
      const items: ShoppingItem[] = [
        { amount: '600', unit: 'g', name: 'flour', recipeName: 'Recipe A' },
        { amount: '600', unit: 'g', name: 'flour', recipeName: 'Recipe B' }
      ]
      const result = formatForClipboard([], items)
      expect(result).toBe('1.2 kg - flour (Recipe A: 600 g, Recipe B: 600 g)')
    })
  })

  describe('formatOrigins', () => {
    it('returns empty string for undefined', () => {
      expect(formatOrigins(undefined)).toBe('')
    })

    it('returns empty string for empty array', () => {
      expect(formatOrigins([])).toBe('')
    })

    it('formats single origin as just the recipe name', () => {
      const origins: ItemOrigin[] = [{ recipeName: 'Pasta', amount: '2', unit: 'cups' }]
      expect(formatOrigins(origins)).toBe(' (Pasta)')
    })

    it('formats single origin with group name', () => {
      const origins: ItemOrigin[] = [
        { recipeName: 'Pasta', amount: '2', unit: 'cups', groupName: 'Sauce' }
      ]
      expect(formatOrigins(origins)).toBe(' (Pasta [Sauce])')
    })

    it('formats multiple origins with name: amount unit per entry', () => {
      const origins: ItemOrigin[] = [
        { recipeName: 'Pasta', amount: '2', unit: 'cups' },
        { recipeName: 'Soup', amount: '1', unit: 'cups' }
      ]
      expect(formatOrigins(origins)).toBe(' (Pasta: 2 cups, Soup: 1 cups)')
    })

    it('formats multiple origins with group names', () => {
      const origins: ItemOrigin[] = [
        { recipeName: 'Pasta', amount: '2', unit: 'cups', groupName: 'Sauce' },
        { recipeName: 'Soup', amount: '1', unit: 'cups', groupName: 'Base' }
      ]
      expect(formatOrigins(origins)).toBe(' (Pasta [Sauce]: 2 cups, Soup [Base]: 1 cups)')
    })

    it('formats mixed origins where only some have group names', () => {
      const origins: ItemOrigin[] = [
        { recipeName: 'Pasta', amount: '2', unit: 'cups', groupName: 'Sauce' },
        { recipeName: 'Soup', amount: '1', unit: 'cups' }
      ]
      expect(formatOrigins(origins)).toBe(' (Pasta [Sauce]: 2 cups, Soup: 1 cups)')
    })
  })

  describe('combining same-name meal plan items', () => {
    it('combines items with the same name and unit from different recipes', () => {
      const items: ShoppingItem[] = [
        { amount: '2', unit: 'cups', name: 'flour', recipeName: 'Recipe A' },
        { amount: '1', unit: 'cups', name: 'flour', recipeName: 'Recipe B' }
      ]
      const result = formatForClipboard([], items)
      expect(result).toBe('3 cups - flour (Recipe A: 2 cups, Recipe B: 1 cups)')
    })

    it('does not combine items with the same name but different units', () => {
      const items: ShoppingItem[] = [
        { amount: '200', unit: 'g', name: 'butter', recipeName: 'Recipe A' },
        { amount: '2', unit: 'tbsp', name: 'butter', recipeName: 'Recipe B' }
      ]
      const result = formatForClipboard([], items)
      expect(result).toBe('200 g - butter (Recipe A)\n2 tbsp - butter (Recipe B)')
    })

    it('does not combine custom items with meal plan items of same name', () => {
      const custom: ShoppingItem[] = [{ id: 'c1', amount: '1', unit: 'cups', name: 'flour' }]
      const meal: ShoppingItem[] = [
        { amount: '2', unit: 'cups', name: 'flour', recipeName: 'Recipe A' }
      ]
      const result = formatForClipboard(custom, meal)
      expect(result).toBe('1 cups - flour\n2 cups - flour (Recipe A)')
    })

    it('treats two custom items with same name independently (no recipeName)', () => {
      const items: ShoppingItem[] = [
        { id: 'a', amount: '1', unit: 'cups', name: 'milk' },
        { id: 'b', amount: '2', unit: 'cups', name: 'milk' }
      ]
      const result = formatForClipboard(items)
      expect(result).toBe('1 cups - milk\n2 cups - milk')
    })

    it('shows single recipe origin for a non-combined meal item', () => {
      const items: ShoppingItem[] = [
        { amount: '3', unit: 'tbsp', name: 'oil', recipeName: 'Stir Fry' }
      ]
      const result = formatForClipboard([], items)
      expect(result).toBe('3 tbsp - oil (Stir Fry)')
    })

    it('includes group name in origin for a single meal item with group', () => {
      const items: ShoppingItem[] = [
        { amount: '3', unit: 'tbsp', name: 'oil', recipeName: 'Stir Fry', groupName: 'Sauce' }
      ]
      const result = formatForClipboard([], items)
      expect(result).toBe('3 tbsp - oil (Stir Fry [Sauce])')
    })

    it('includes group names in origins when combining items from multiple recipes', () => {
      const items: ShoppingItem[] = [
        { amount: '2', unit: 'cups', name: 'flour', recipeName: 'Cake', groupName: 'Dry' },
        { amount: '1', unit: 'cups', name: 'flour', recipeName: 'Bread', groupName: 'Base' }
      ]
      const result = formatForClipboard([], items)
      expect(result).toBe('3 cups - flour (Cake [Dry]: 2 cups, Bread [Base]: 1 cups)')
    })

    it('combines case-insensitively by name', () => {
      const items: ShoppingItem[] = [
        { amount: '1', unit: 'cup', name: 'Rice', recipeName: 'Recipe A' },
        { amount: '2', unit: 'cup', name: 'rice', recipeName: 'Recipe B' }
      ]
      const result = formatForClipboard([], items)
      expect(result).toBe('3 cup - Rice (Recipe A: 1 cup, Recipe B: 2 cup)')
    })

    it('falls back to first amount when amounts are non-numeric', () => {
      const items: ShoppingItem[] = [
        { amount: 'some', unit: 'tbsp', name: 'spice', recipeName: 'Recipe A' },
        { amount: 'a bit', unit: 'tbsp', name: 'spice', recipeName: 'Recipe B' }
      ]
      const result = formatForClipboard([], items)
      expect(result).toBe('some tbsp - spice (Recipe A: some tbsp, Recipe B: a bit tbsp)')
    })
  })

  describe('prepareForExport', () => {
    it('returns merged and combined items', () => {
      const custom: ShoppingItem[] = [{ id: 'c1', amount: '1', unit: 'L', name: 'milk' }]
      const meal: ShoppingItem[] = [
        { amount: '2', unit: 'cups', name: 'flour', recipeName: 'Cake' },
        { amount: '1', unit: 'cups', name: 'flour', recipeName: 'Bread' }
      ]
      const result = prepareForExport(custom, meal)
      expect(result).toHaveLength(2)
      expect(result[0]).toMatchObject({ name: 'milk', amount: '1', unit: 'L' })
      expect(result[1]).toMatchObject({ name: 'flour', amount: '3', unit: 'cups' })
      expect(result[1].origins).toHaveLength(2)
    })

    it('returns custom items only when no meal items given', () => {
      const custom: ShoppingItem[] = [{ id: 'c1', amount: '1', unit: 'L', name: 'milk' }]
      const result = prepareForExport(custom)
      expect(result).toHaveLength(1)
      expect(result[0]).toMatchObject({ name: 'milk' })
    })
  })

  describe('formatForClipboard', () => {
    it('formats custom items only when no meal items given', () => {
      const result = formatForClipboard(customItems)
      expect(result).toBe('0.5 tsp - salt')
    })

    it('merges custom and meal plan items into a single flat list', () => {
      const result = formatForClipboard(customItems, mealItems)
      expect(result).toBe('0.5 tsp - salt\n2 cups - flour\n1 tbsp - sugar\n100 g - butter')
    })

    it('omits meal plan items when meal items array is empty', () => {
      const result = formatForClipboard(customItems, [])
      expect(result).toBe('0.5 tsp - salt')
    })

    it('formats meal items only when custom items are empty', () => {
      const result = formatForClipboard([], mealItems)
      expect(result).toBe('2 cups - flour\n1 tbsp - sugar\n100 g - butter')
    })

    it('returns empty string when both are empty', () => {
      expect(formatForClipboard([], [])).toBe('')
    })
  })

  describe('formatForAppleNotes', () => {
    const fixedDate = new Date(2026, 4, 26)

    it('includes date title', () => {
      const result = formatForAppleNotes(customItems, undefined, fixedDate)
      expect(result.startsWith('Shopping list 26/05/2026')).toBe(true)
    })

    it('merges custom and meal plan items into a single flat list after title', () => {
      const result = formatForAppleNotes(customItems, mealItems, fixedDate)
      expect(result).toBe(
        'Shopping list 26/05/2026\n\n0.5 tsp salt\n2 cups flour\n1 tbsp sugar\n100 g butter'
      )
    })

    it('returns just the title when both are empty', () => {
      const result = formatForAppleNotes([], [], fixedDate)
      expect(result).toBe('Shopping list 26/05/2026')
    })

    it('formats meal items only when custom items are empty', () => {
      const result = formatForAppleNotes([], mealItems, fixedDate)
      expect(result).toBe('Shopping list 26/05/2026\n\n2 cups flour\n1 tbsp sugar\n100 g butter')
    })

    it('includes origin in Apple Notes format', () => {
      const meal: ShoppingItem[] = [
        { amount: '2', unit: 'cups', name: 'flour', recipeName: 'Cake' }
      ]
      const result = formatForAppleNotes([], meal, fixedDate)
      expect(result).toBe('Shopping list 26/05/2026\n\n2 cups flour (Cake)')
    })
  })

  describe('formatAsTxt', () => {
    it('produces same output as clipboard format', () => {
      expect(formatAsTxt(customItems, mealItems)).toBe(formatForClipboard(customItems, mealItems))
    })

    it('produces same output without meal items', () => {
      expect(formatAsTxt(customItems)).toBe(formatForClipboard(customItems))
    })
  })

  describe('groupByStore', () => {
    const orderedCategories: Category[] = [
      { id: 'cat-produce', name: 'Produce' },
      { id: 'cat-baking', name: 'Baking' }
    ]
    // salt -> Baking, flour -> Baking, sugar -> Produce (contrived), butter unmapped
    const nameToCategoryId: Record<string, string> = {
      salt: 'cat-baking',
      flour: 'cat-baking',
      sugar: 'cat-produce'
    }

    it('orders groups by the store order and reports items with no category as uncategorized', () => {
      const { groups, uncategorized, unordered } = groupByStore(
        customItems,
        mealItems,
        orderedCategories,
        nameToCategoryId
      )
      expect(groups.map((g) => g.category)).toEqual(['Produce', 'Baking'])
      expect(groups[0].items.map((i) => i.name)).toEqual(['sugar'])
      expect(groups[1].items.map((i) => i.name)).toEqual(['salt', 'flour'])
      // butter maps to no category at all
      expect(uncategorized.map((i) => i.name)).toEqual(['butter'])
      expect(unordered).toEqual([])
    })

    it('reports items whose category is not part of the store as unordered', () => {
      const items: ShoppingItem[] = [
        { amount: '1', unit: '', name: 'frozen pizza' },
        { amount: '2', unit: '', name: 'apples' }
      ]
      // frozen pizza has a real category, but the store does not order "cat-frozen"
      const { groups, uncategorized, unordered } = groupByStore(
        items,
        undefined,
        orderedCategories,
        {
          'frozen pizza': 'cat-frozen',
          apples: 'cat-produce'
        }
      )
      expect(groups.map((g) => g.category)).toEqual(['Produce'])
      expect(groups[0].items.map((i) => i.name)).toEqual(['apples'])
      expect(unordered.map((i) => i.name)).toEqual(['frozen pizza'])
      expect(uncategorized).toEqual([])
    })

    it('omits categories that have no items', () => {
      const { groups, uncategorized } = groupByStore([], mealItems, orderedCategories, {
        flour: 'cat-baking'
      })
      expect(groups.map((g) => g.category)).toEqual(['Baking'])
      expect(uncategorized.map((i) => i.name)).toEqual(['sugar', 'butter'])
    })

    it('matches category by normalized (trimmed, lowercased) name', () => {
      const items: ShoppingItem[] = [{ amount: '1', unit: '', name: '  SALT  ' }]
      const { groups } = groupByStore(items, undefined, orderedCategories, { salt: 'cat-baking' })
      expect(groups).toEqual([
        { category: 'Baking', items: [{ amount: '1', unit: '', name: '  SALT  ' }] }
      ])
    })

    it('returns empty buckets when there are no items', () => {
      expect(groupByStore([], [], orderedCategories, nameToCategoryId)).toEqual({
        groups: [],
        uncategorized: [],
        unordered: []
      })
    })

    it('applies unit upgrades inside groups', () => {
      const items: ShoppingItem[] = [{ amount: '1000', unit: 'g', name: 'flour' }]
      const { groups } = groupByStore(items, undefined, orderedCategories, { flour: 'cat-baking' })
      expect(groups[0].items[0]).toMatchObject({ amount: '1', unit: 'kg' })
    })

    it('combines same-name meal items within a store group', () => {
      const meal: ShoppingItem[] = [
        { amount: '1', unit: 'cups', name: 'flour', recipeName: 'Cake' },
        { amount: '2', unit: 'cups', name: 'flour', recipeName: 'Bread' }
      ]
      const { groups } = groupByStore([], meal, orderedCategories, { flour: 'cat-baking' })
      const flourItems = groups.find((g) => g.category === 'Baking')?.items ?? []
      expect(flourItems).toHaveLength(1)
      expect(flourItems[0]).toMatchObject({ amount: '3', unit: 'cups', name: 'flour' })
      expect(flourItems[0].origins).toHaveLength(2)
    })
  })

  describe('toExportGroups', () => {
    it('returns just the store groups when nothing is left over', () => {
      const grouping = {
        groups: [{ category: 'Produce', items: [{ amount: '2', unit: '', name: 'apples' }] }],
        uncategorized: [],
        unordered: []
      }
      expect(toExportGroups(grouping)).toEqual(grouping.groups)
    })

    it('appends a single trailing Other group combining uncategorized and unordered items', () => {
      const grouping = {
        groups: [{ category: 'Produce', items: [{ amount: '2', unit: '', name: 'apples' }] }],
        uncategorized: [{ amount: '1', unit: '', name: 'butter' }],
        unordered: [{ amount: '1', unit: '', name: 'frozen pizza' }]
      }
      expect(toExportGroups(grouping)).toEqual([
        ...grouping.groups,
        {
          category: 'Other',
          items: [
            { amount: '1', unit: '', name: 'butter' },
            { amount: '1', unit: '', name: 'frozen pizza' }
          ]
        }
      ])
    })
  })

  describe('grouped formatters', () => {
    const groups = [
      { category: 'Produce', items: [{ amount: '2', unit: '', name: 'apples' }] },
      { category: 'Other', items: [{ amount: '1', unit: 'tub', name: 'icecream' }] }
    ]

    it('formatGroupedForClipboard renders headers and items', () => {
      expect(formatGroupedForClipboard(groups)).toBe(
        'Produce:\n2  - apples\n\nOther:\n1 tub - icecream'
      )
    })

    it('formatGroupedAsTxt matches clipboard output', () => {
      expect(formatGroupedAsTxt(groups)).toBe(formatGroupedForClipboard(groups))
    })

    it('formatGroupedForAppleNotes includes the date title', () => {
      const result = formatGroupedForAppleNotes(groups, new Date(2026, 4, 26))
      expect(result).toBe(
        'Shopping list 26/05/2026\n\nProduce:\n2  apples\n\nOther:\n1 tub icecream'
      )
    })

    it('formatGroupedForAppleNotes returns just the title when there are no groups', () => {
      expect(formatGroupedForAppleNotes([], new Date(2026, 4, 26))).toBe('Shopping list 26/05/2026')
    })

    it('formatGroupedForClipboard appends origins for items with origins', () => {
      const groupsWithOrigins = [
        {
          category: 'Baking',
          items: [
            {
              amount: '3',
              unit: 'cups',
              name: 'flour',
              origins: [
                { recipeName: 'Cake', amount: '2', unit: 'cups' },
                { recipeName: 'Bread', amount: '1', unit: 'cups' }
              ]
            }
          ]
        }
      ]
      expect(formatGroupedForClipboard(groupsWithOrigins)).toBe(
        'Baking:\n3 cups - flour (Cake: 2 cups, Bread: 1 cups)'
      )
    })
  })
})
