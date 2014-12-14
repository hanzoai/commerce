ProductView = require '../views/product'
Validation = require '../../utils/validation'

exports.setupFormValidation = (formId)->
  ->
    minimumPasswordLength = 6
    $form = $(formId)
    $form.find('input, select, textarea').click ->
      $(@).removeClass('error')

    $form.submit ->
      valid = true
      errors = []

      # Get all inputs that are visible and empty
      empty = $form.find('div:visible.required > input').filter ->
        Validation.isEmpty $(@).val()

      email = $form.find('input[name="User.Email"], input[name="Email"]')
      if email.length != 0
        unless Validation.isEmail email.val()
          valid = false
          Validation.error email
          errors.push "Invalid email."

      oldPassword = $form.find('input[name="OldPassword"]')
      if oldPassword.length != 0
        if !Validation.isPassword oldPassword.val(), minimumPasswordLength
          valid = false
          Validation.error oldPassword
          errors.push "Password must be at least #{minimumPasswordLength} characters long"

      password = $form.find('input[name="Password"]')
      if password.length != 0
        if !Validation.isPassword password.val(), minimumPasswordLength
          valid = false
          Validation.error password
          errors.push "Password must be at least #{minimumPasswordLength} characters long"
        else
          confirmPassword = $form.find('input[name="ConfirmPassword"]')
          if confirmPassword.length != 0
            unless Validation.passwordsMatch(password.val(), confirmPassword.val())
              valid = false
              Validation.error confirmPassword
              errors.push "Passwords must match"

      if empty.length > 0
        valid = false
        Validation.error empty
        missing = (v.toLowerCase().trim() for v in empty.parent().text().split('\n') when v.trim())
        if missing.length > 1
          errors.push "Please enter your #{missing.slice(0, -1).join(', ') + (if missing.length == 2 then '' else ',') + " and " + missing.slice(-1)}."
        else
          errors.push "Please enter your #{missing[0]}"

      unless valid
        $errors = $form.find('.errors')
        $errors.text ''

        # display errors
        for error in errors
          $errors.append $("<p>#{error}</p>")

      return valid

exports.setupViews = ->
  console.log 'store#setupViews'
  console.log 'hi'
  for div in $('.product-text')
    do (div) ->
      console.log 'product'
      view = new ProductView el: $(div)
      window.view = view
      app.views.push view
      view.bind()
      view.render()

# Simple thumbnail gallery
exports.gallery = ->
  fading = false
  $(document).ready ->
    $('.product-viewer .gallery .thumbnail').on 'click', ->
      if !fading
        fading = true
        thumbnail = $(@)
        gallery = thumbnail.parent()
        viewer = gallery.parent()
        images = viewer.find '.preview'

        gallery.children().removeClass 'selected'
        thumbnail.addClass 'selected'

        i = $(@).index()
        images.hide()
        $(images[i]).show()

        setTimeout ->
          fading = false
        , 300

exports.setupStylesAndSizes =->
  $(document).ready ->
    $('.size').val('M')
    $('.style').on 'change', ->
      style = $(@)
      config = style.parent()

      size = config.find('.size')

      if style.val() == "Men's T-Shirt"
        hasOptions = false
        size.find('option').each ->
          option = $(@)
          hasOptions = true if option.val() == 'XXL' || option.val() == 'XXXL'
        if !hasOptions
          size.append $('<option value="XXL">XXL</option>')
          size.append $('<option value="XXXL">XXXL</option>')
      else if style.val() =="Women's T-Shirt"
        size.find('option').each ->
          option = $(@)
          option.remove() if option.val() == 'XXL' || option.val() == 'XXXL'

# Swap AR-1 helmets when color selected
exports.customizeAr1 = ->
  $slides = ($(i) for i in $('#productSlideshow .slide img'))

  $('[data-variant-option-name=Color]').change ->
    if $(@).val() is "Black"
      $slides[0].fadeIn()
      $slides[1].fadeOut()
    else
      $slides[1].fadeIn()
      $slides[0].fadeOut()

exports.menu = ->
  $('.menu-icon').click ->
    $('body').toggleClass('mobile')
