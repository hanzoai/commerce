# Determine if selected options match variant
optionsMatch = (options, variant) ->
  for option, value of options
    if variant[option] != value
      return false
  true

# Determine variant selected for product
exports.getVariant = (slug, options) ->
  # If called with only one argument, assume it's options
  unless options?
    [options, slug] = [slug, null]

  # If we get a slug we're on preorder page and need to use AllProducts
  if slug?
    variants = AllProducts[slug].Variants
  else
    variants = currentProduct.Variants

  # Figure out SKU, all options match match variant
  for variant in variants
    return variant if optionsMatch options, variant

  # Quick sanity check
  if (Object.keys variant).length > 0
    throw new Error 'Mismatch between product and options'

  # Only one variant, no options.
  variants[0]
