/* global csio */

// Globals
window.csio = window.csio || {};

$('div.field').on('click', function() {
    $(this).removeClass('error');
});

$('#form').submit(function(e) {
    var empty = $('div.required > input').filter(function() {return $(this).val() === '';});
    if (empty.length > 0) {
        e.preventDefault();
        empty.parent().addClass('error');
    }
});


// Show payment options when first half is competed.
$(document).ready(function() {
  var $required = $('div:visible.required > input')

  var showPaymentOptions = $.debounce(250, function() {
    console.log('checking for completion...')

    for (var i=0; i< $required.length; i++) {
      if ($required[i].value === '') return
    }

    var fieldset = $('div.sqs-checkout-form-payment-content > fieldset');
    fieldset.css('display', 'block');
    fieldset.css('opacity', '0');
    fieldset.fadeTo(1000, 1);

    $required.unbind('keyup', showPaymentOptions)
  })

  $required.bind('keyup', showPaymentOptions)
})

$('#form').card({
    container: '#card-wrapper',
    numberInput: 'input[name="Order.Account.Number"]',
    expiryInput: 'input[name="RawExpiry"]',
    cvcInput: 'input[name="Order.Account.CVV2"]',
    nameInput: 'input[name="User.FirstName"], input[name="User.LastName"]'
});

$('input[name="ShipToBilling"]').change(function(){
    var fields = $('#shipping_fields')
    if (this.checked) {
        fields.css('display', 'block');
    } else {
        fields.css('display', 'none');
    }
});
