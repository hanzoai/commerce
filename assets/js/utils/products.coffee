# Determine if selected options match variant
optionsMatch = (options, variant) ->
  for option, value of options
    if variant[option] != value
      return false
  true

# determin variant selected for product
exports.getVariant = (options, slug) ->
  if slug?
    variants = allProducts[slug].Variants
  else
    variants = currentProduct.Variants

  console.log 'getVariant', options, variants
  window.options = options
  window.variants = variants

  # Figure out SKU, all options match match variant
  for variant in variants
    return variant if optionsMatch options, variant

  # Only one variant, no options.
  variants[0]
