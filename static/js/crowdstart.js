// Globals
/* global csio */
window.csio = window.csio || {};
csio.cookieName = 'SKULLYSystemsCart';

// Enable JSON cookies
$.cookie.json = true;

// Helper functions
function humanizeNumber(num) {
  return num.toString().replace(/(\d)(?=(\d\d\d)+(?!\d))/g, '$1,');
}

function formatCurrency(num) {
  var currency = num || 0;

  return humanizeNumber(currency.toFixed(2));
}

csio.Alert = function(opts) {
  var $el = $('.sqs-widgets-confirmation.alert')
  var offset = opts.$nextTo.offset()
  var topOffset = offset.top - $(window).scrollTop()

  $el.find('.title').text(opts.title)
  $el.find('.message').text(opts.message)
  $el.find('.confirmation-button').text(opts.confirm)

  $el.css({
    position: 'fixed',
    top:      (topOffset - 42) + 'px',
    left:     (offset.left - 66) + 'px',
  })

  // Show
  $el.fadeIn(200)

  // Dismiss
  function dismiss() {
    $el.fadeOut(200, function() {
      $el.css({top: -1000})
    })
  }

  // Dismiss on click, escape, and scroll
  $(document).mousedown(function() {
    dismiss()
  })

  $(document).keydown(function(e) {
    if (!e) e = event;
    if (e.keyCode == 27) dismiss()
  })

  $(window).scroll(function() {
    dismiss()
  })
}

// Lookup variant based on selected options.
csio.getVariant = function() {
  var selected       = {};
  var variants       = csio.currentProduct.Variants;
  var missingOptions = [];

  // Determine if selected options match variant
  function optionsMatch(selected, variant) {
    for (var k in selected)
      if (variant[k] !== selected[k])
        return false
    return true
  }

  // Get selected options
  $('.variant-option').each(function(i, v) {
    $(v).find('select').each(function(i,v) {
      var $select = $(v);
      var name  = $select.data('variant-option-name');
      var value = $select.val();
      selected[name] = value;

      if (value === 'none') missingOptions.push(name)
    });
  });

  // Warn if missing options (we'll be unable to figure out a SKU).
  if (missingOptions.length > 0) {
    return csio.Alert({
      title:   'Unable To Add Item',
      message: 'Please select a ' + missingOptions[0] + ' option.',
      confirm: 'Okay',
      $nextTo: $('.sqs-add-to-cart-button'),
    })
  }

  // Figure out SKU
  for (var i=0; i<variants.length; i++) {
    var variant = variants[i];

    // All options match match variant
    if (optionsMatch(selected, variant)) return variant
  }

  // Only one variant, no options.
  return variants[0];
};

// Add to cart
csio.addToCart = function() {
  var quantity = parseInt($('#quantity').val(), 10);
  var cart     = $.cookie(csio.cookieName) || {};
  var variant  = csio.getVariant();

  if (variant == null) return

  if (cart[variant.SKU]) {
    cart[variant.SKU].quantity += quantity;
  } else {
    cart[variant.SKU] = {
      sku:      variant.SKU,
      color:    variant.Color,
      img:      csio.currentProduct.Images[0].Url,
      name:     csio.currentProduct.Title,
      quantity: quantity,
      size:     variant.Size,
      price:    parseInt(variant.Price, 10) * 0.0001,
      slug:     csio.currentProduct.Slug,
    };
  }

  // Set cookie
  csio.setCart(cart);

  // Update cart hover
  csio.updateCartHover(cart);
};

csio.setCart = function(cart) {
  $.cookie(csio.cookieName, cart, { expires: 30, path: '/' });
};

csio.getCart = function() {
  return $.cookie(csio.cookieName) || {};
};

csio.clearCart = function () {
  $.cookie(csio.cookieName, {}, { expires: 30, path: '/' });
};

csio.updateCartHover = function(modifiedCart) {
  var cart = modifiedCart || csio.getCart();
  var numItems = 0;
  var subTotal = 0;

  for (var k in cart) {
    var lineItem = cart[k];
    numItems += lineItem.quantity;
    subTotal += lineItem.price * lineItem.quantity;
  }

  $('.total-quantity').text(humanizeNumber(numItems));
  $('.subtotal .price span').text(formatCurrency(subTotal));

  if (numItems === 1)
    $('.details span.suffix').text('item')
  else
    $('.details span.suffix').text('items')

};

// Events

// Show cart when cart button is clicked
$('.fixed-cart').click(function() {
  window.location = '/cart';
})

// Hide cart button on cart page
if (location.pathname == '/cart') {
  $('.fixed-cart').hide()
}

// Product gallery image switching
$('#productThumbnails .slide img').each(function(i,v) {
  $(v).click(function(){
    var src = $(v).data('src');
    $('#productSlideshow .slide img').each(function(i,v) {
      if (src === $(v).data('src'))
        $(v).fadeIn(400)
      else
        $(v).fadeOut(400)
    })
  })
})

// Update cart hover onload
csio.updateCartHover()
