PerkView     = require '../views/perk'
HelmetView   = require '../views/helmet'
GearView = require '../views/gear'
HatsView     = require '../views/hats'
ShippingView = require '../views/shipping'
EventEmitter = require 'mvstar/lib/event-emitter'

exports.displayPerks = ->
  console.log 'displaying perks'
  perkMap = {}

  window.helmetTotal = 0
  window.gearTotal = 0

  for contribution in PreorderData.contributions
    unless (view = perkMap[contribution.Perk.Id])?
      view = new PerkView state: contribution.Perk
      view.set 'count', 1
      view.render()
      $('.perk').append view.$el
      perkMap[contribution.Perk.Id] = view
    else
      view.set 'count', (view.get 'count') + 1

    window.helmetTotal += parseInt(contribution.Perk.HelmetQuantity, 10)
    window.gearTotal += parseInt(contribution.Perk.GearQuantity, 10)

  return

exports.displayHelmets = ->
  console.log 'displaying helmets'
  return unless window.helmetTotal > 0

  view = new HelmetView
    state:
      total: window.helmetTotal
  view.render()
  view.bind()

  # First time through, no existing order, use defaults
  if (not PreorderData.hasPassword) or
     (not PreorderData.existingOrder.Items?) or
     (PreorderData.existingOrder.length == 0)
    view.newItem()
  else
    # Get variants
    variants = {}
    for variant in allProducts['ar-1'].Variants
      variants[variant.SKU] = variant

    first = true
    hasItem = false

    # Restore order
    for item in PreorderData.existingOrder.Items
      if item.Slug == 'ar-1'
        itemView = view.newItem()
        itemView.set 'quantity', item.Quantity
        itemView.set 'sku',      item.SKU
        itemView.set 'color',    variants[item.SKU].Color
        itemView.set 'size',     variants[item.SKU].Size
        itemView.updateQuantity()

        if first
          view.set 'color', variants[item.SKU].Color
          first = false
        hasItem = true

    view.newItem() unless hasItem

  $('.item.helmet').append view.$el

exports.displayApparel = ->
  console.log 'displaying apparel'
  return unless window.gearTotal > 0

  view = new GearView
    state:
      total: window.gearTotal
  view.render()
  view.bind()

  if (not PreorderData.hasPassword) or
     (not PreorderData.existingOrder.Items?) or
     (PreorderData.existingOrder.length == 0)
    view.newItem()
  else
    # Get variants
    variants = {}
    for variant in allProducts['t-shirt'].Variants
      variants[variant.SKU] = variant

    hasItem = false
    # Restore order
    for item in PreorderData.existingOrder.Items
      if item.Slug == 't-shirt'
        console.log item
        itemView = view.newItem()
        itemView.set 'quantity', item.Quantity
        itemView.set 'sku',      item.SKU
        itemView.set 'style',    variants[item.SKU].Style
        itemView.set 'size',     variants[item.SKU].Size
        itemView.updateQuantity()
        hasItem = true

    view.newItem() unless hasItem

  $('.item.gear').append view.$el

exports.displayHats = ->
  console.log 'displaying hats'
  return unless window.gearTotal > 0

  view = new HatsView
    state:
      total: window.gearTotal
  view.render()
  view.bind()

  if (not PreorderData.hasPassword) or
     (not PreorderData.existingOrder.Items?) or
     (PreorderData.existingOrder.length == 0)
    view.newItem()
  else
    # Get variants
    variants = {}
    for variant in allProducts['hat'].Variants
      variants[variant.SKU] = variant

    hasItem = false
    # Restore order
    for item in PreorderData.existingOrder.Items
      if item.Slug == 'hat'
        console.log item
        itemView = view.newItem()
        itemView.set 'quantity', item.Quantity
        itemView.set 'sku',      item.SKU
        itemView.set 'size',     variants[item.SKU].Size
        itemView.updateQuantity()
        hasItem = true

    view.newItem() unless hasItem


  $('.item.hats').append view.$el

exports.initializeShipping = ->
  console.log 'initializing shipping'
  view = new ShippingView
    state: $.extend {}, PreorderData.user, PreorderData.user.ShippingAddress
  console.log 'country', view.get 'country'
  view.render()
  view.bind()
  $('#skully .shipping .form').append(view.$el)
  return
