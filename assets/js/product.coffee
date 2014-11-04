alert = require './alert'

# Lookup variant based on selected options.
exports.getVariant = ->
  selected = {}
  variants = csio.currentProduct.Variants
  missingOptions = []

  # Determine if selected options match variant
  optionsMatch = (selected, variant) ->
    for k,v of selected
      if variant[k] != selected[k]
        return false
    true

  # Get selected options
  $(".variant-option").each (i, v) ->
    $(v).find("select").each (i, v) ->
      $select = $(v)
      name = $select.data("variant-option-name")
      value = $select.val()
      selected[name] = value
      missingOptions.push name  if value is "none"
      return

    return

  # Warn if missing options (we'll be unable to figure out a SKU).
  if missingOptions.length > 0
    return alert({
      title: "Unable To Add Item"
      message: "Please select a " + missingOptions[0] + " option."
      confirm: "Okay"
      nextTo: ".sqs-add-to-cart-button"
    }).show()

  # Figure out SKU
  for variant in variants
    # All options match match variant
    return variant if optionsMatch selected, variant

  # Only one variant, no options.
  variants[0]
