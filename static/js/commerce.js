// Globals
csio = window.csio || {};
csio.cookieName = 'SKULLYSystemsCart';

// JSON cookies
$.cookie.json = true;

// Show cart when cart button is clicked
$('.fixed-cart').click(function() {
  window.location = '/cart';
})

// Hide cart button on cart page
if (location.pathname == '/cart') {
  $('.fixed-cart').hide()
}

function humanizeNumber(num) {
  var num = (num || 0) + "";

  return num.replace(/(\d)(?=(\d\d\d)+(?!\d))/g, '$1,');
}

function formatCurrency(num) {
  var num = num || 0;

  return '$' + humanizeNumber(num.toFixed(2))
}

// Lookup variant based on selected options.
csio.getVariant = function() {
  var options = {};
  var variants = csio.currentProduct.Variants;

  $('.variant-option').each(function(i, v) {
    $(v).find('select').each(function(i,v) {
      var $select = $(v);
      var name  = $select.data('variant-option-name');
      var value = $select.val();
      options[name] = value;
    })
  })

  for (var k in options) {
    if (options[k] === "none")
      return
  }

  for (var i=0; i<csio.currentProduct.Variants.length; i++) {
    var variant = variants[i];
    for (k in options) {
      if (variant[k] != options[k])
        continue

      return variant
    }
  }
}

csio.addToCart = function() {
  var quantity     = parseInt($('#quantity').val(), 10);
  var cart         = $.cookie(csio.cookieName) || {};
  var variant      = csio.getVariant();

  if (variant == null) {
    alert('Please select an option')
    return
  }

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
      price:    variant.Price*0.0001,
      slug:     csio.currentProduct.Slug,
    }
  }

  // Set cookie
  $.cookie(csio.cookieName, cart, { expires: 30, path: '/' });

  // Update cart hover
  csio.updateCartHover(cart)
}

csio.getCart = function() {
  return $.cookie(csio.cookieName) || {};
}

csio.clearCart = function () {
  $.cookie(csio.cookieName, {}, { expires: 30, path: '/' });
}

csio.updateCartHover = function(cart) {
  var cart = cart || csio.getCart();
  var numItems = 0;
  var subTotal = 0;

  for (var k in cart) {
    var lineItem = cart[k];
    numItems += lineItem.quantity;
    subTotal += lineItem.price * lineItem.quantity;
  }

  $('.total-quantity').text(humanizeNumber(numItems))
  $('.subtotal').text(formatCurrency(subTotal))
}

// Update cart hover onload
csio.updateCartHover()
