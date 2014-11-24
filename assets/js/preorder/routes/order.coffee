PerkView = require '../views/perk'
HelmetView = require '../views/helmet'
GearView = require '../views/gear'
HatsView    = require '../views/hats'
ShippingView = require '../views/shipping'
EventEmitter = require 'mvstar/lib/event-emitter'

exports.setupView = ->

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
  if window.helmetTotal > 0
    view = new HelmetView {state: {total: window.helmetTotal}, emitter: new EventEmitter }
    view.render()
    view.bind()
    view.newItem()
    $('.item.helmet').append view.$el
  return

exports.displayApparel = ->
  console.log 'displaying apparel'
  if window.gearTotal > 0
    view = new GearView {state: {total: window.gearTotal}, emitter: new EventEmitter }
    view.render()
    view.bind()
    view.newItem()
    $('.item.gear').append view.$el
  return

exports.displayHats = ->
  if window.gearTotal > 0
    console.log 'displaying hats'
    view = new HatsView {state: {total: window.gearTotal}, emitter: new EventEmitter }
    view.render()
    view.bind()
    view.newItem()
    $('.item.hats').append view.$el
  return

exports.initializeShipping = ->
  console.log 'initializing shipping'
  view = new ShippingView {state: $.extend {}, PreorderData.user, PreorderData.user.ShippingAddress }
  view.render()
  view.bind()
  $('#skully .shipping .form').append(view.$el)
  return
