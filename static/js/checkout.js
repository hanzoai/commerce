/* global csio */

// Globals
window.csio = window.csio || {};

// Validation helper
var validation = {
    isEmpty: function (str) {
        return str.trim().length === 0;
    },
    isEmail: function(email) {
        var pattern = new RegExp(/^[+a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,4}$/i);
        return pattern.test(email);
    }
}

$('div.field').on('click', function() {
    $(this).removeClass('error');
});

$('#form').submit(function(e) {
    var empty = $('div:visible.required > input').filter(function() {return $(this).val() === '';});

    var email = $('input[name="User.Email"]')
    if (!validation.isEmail(email.val())) {
        console.log(validation.isEmail(email.text()));
        e.preventDefault();
        email.parent().addClass('error');
        email.parent().addClass('shake');
        setTimeout(function(){
            email.parent().removeClass('shake');
        }, 500);
    }

    if (empty.length > 0) {
        e.preventDefault();
        empty.parent().addClass('error');
        empty.parent().addClass('shake');

        setTimeout(function(){
            empty.parent().removeClass('shake');
        }, 500);
    }
});

// Show payment options when first half is competed.
var $requiredVisible = $('div:visible.required > input')

var showPaymentOptions = $.debounce(250, function() {
  // Check if all required inputs are filled
  for (var i=0; i< $requiredVisible.length; i++) {
    if ($requiredVisible[i].value === '') return
  }

  var fieldset = $('div.sqs-checkout-form-payment-content > fieldset');
  fieldset.css('display', 'block');
  fieldset.css('opacity', '0');
  fieldset.fadeTo(1000, 1);

  $requiredVisible.off('keyup', showPaymentOptions)
})

$requiredVisible.on('keyup', showPaymentOptions)

$('#form').card({
    container: '#card-wrapper',
    numberInput: 'input[name="Order.Account.Number"]',
    expiryInput: 'input[name="RawExpiry"]',
    cvcInput: 'input[name="Order.Account.CVV2"]',
    nameInput: 'input[name="User.FirstName"], input[name="User.LastName"]'
});

$('input[name="ShipToBilling"]').change(function(){
    var shipping = $('#shippingInfo')
    if (this.checked) {
        shipping.fadeOut(500);
        setTimeout(function(){
            shipping.css('display', 'none');
        }, 500);
    } else {
        shipping.fadeIn(500);
        shipping.css('display', 'block');
    }
});

// Update tax display
var $state = $('select[name="Order.BillingAddress.State"]');
var $city = $('input[name="Order.BillingAddress.City"]');
var $tax = $('div.tax.total > div.price > span');
var $total = $('div.grand-total.total > div.price > span');
var $subtotal = $('div.subtotal.total > div.price > span');

var updateTax = $.debounce(250, function() {
    var city = $city.val();
    var state = $state.val();
    var tax = 0;
    var total = 0;
    var subtotal = parseFloat($subtotal
                              .text()
                              .replace(',', ''));


    // Add CA tax
    if (state === 'CA')
        tax += subtotal * 0.075;

    // Add SF county tax
    if (state == 'CA' && (/san francisco/i).test(city))
        tax += subtotal * 0.0125;

    total = subtotal + tax;

    $tax.text(tax.toFixed(2));
    $total.text(total.toFixed(2));
})

$state.change(updateTax);
$city.on('keyup', updateTax);

// AJAX form submit
csio.handleSubmit = function(formSelector) {
    
}

csio.handleSubmit('#form')
