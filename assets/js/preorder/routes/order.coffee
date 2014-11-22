PerkView = require '../views/perk'
HelmetView = require '../views/helmet'

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
  if window.helmetTotal > 0
    view = new HelmetView {state: {total: window.helmetTotal}}
    view.render()
    view.newItem()
    $('.item.ar1').append view.$el
  return
