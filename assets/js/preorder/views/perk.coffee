View = require 'mvstar/lib/view'

class PerkView extends View
  template: "#perk-template"

  bindings:
    Title:             'h3 span.title'
    Description:       'p.p1'
    EstimatedDelivery: 'p.p2'
    count:             'h3 span.count'

  formatters:
    count: (v) ->
      if v > 1
        " [x#{v}]"
      else
        ''

module.exports = PerkView
