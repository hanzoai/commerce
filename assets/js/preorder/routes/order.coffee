PerkView = require '../views/perk'

exports.setupView = ->

exports.displayPerks = ->
  console.log 'displaying perks'
  perkMap = {}

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
