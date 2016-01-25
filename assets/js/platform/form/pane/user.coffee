_ = require 'underscore'
moment = require 'moment'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
Pane = require './pane'

#TODO: Add actual localization stuff to util
localizeDate = (date)->
  tokens = date.split '/'
  return moment((tokens[2] ? '2015') + ' ' + (tokens[0] ? '01') + ' ' + (tokens[1] ? '01'), 'YYYY-MM-DD').format 'YYYY-MM-DD'

class UserFilterPane extends Pane
  tag: 'user-filter-pane'
  html: require '../../templates/backend/form/pane/user.html'
  path: 'search/user'

  inputConfigs: [
    input('email', 'Email')
    input('firstName', 'First Name')
    input('lastName', 'Last Name')
    input('line1', 'Street Address')
    input('line2', 'Apartment/Suite')
    input('city', 'City')
    input('state', 'State')
    input('postal', 'Postal/ZIP')
    input('minDate', '', 'date-picker')
    input('maxDate', '', 'date-picker')
    input('country', '', 'country-select')
  ]

  js: ()->
    @model =
      email:        ''
      firstName:    ''
      lastName:     ''
      line1:        ''
      line2:        ''
      city:         ''
      state:        ''
      postal:       ''
      country:      '_any'
      minDate:      '01/01/2015'
      maxDate:      moment().format 'L'

    super

  queryString: ()->
    minDate = localizeDate(@model.minDate)
    maxDate = localizeDate(@model.maxDate)

    if moment(minDate, 'YYYY-MM-DD').isAfter moment(maxDate, 'YYYY-MM-DD')
      swap  = maxDate
      maxDate = minDate
      minDate = swap2

    riot.update()

    minDateStr = moment(minDate, 'YYYY-MM-DD').format 'YYYY-MM-DD'
    maxDateStr = moment(maxDate, 'YYYY-MM-DD').format 'YYYY-MM-DD'

    query = "CreatedAt >= #{encodeURIComponent minDateStr} AND CreatedAt <= #{encodeURIComponent maxDateStr}"
    if @model.email
      query += " AND Email = \"#{ encodeURIComponent @model.email }\""

    if @model.firstName
      query += " AND FirstName = \"#{ encodeURIComponent @model.firstName }\""

    if @model.lastName
      query += " AND LastName = \"#{ encodeURIComponent @model.lastName }\""

    if @model.line1
      query += " AND ShippingAddressLine1 = \"#{ encodeURIComponent @model.line1 }\""

    if @model.line2
      query += " AND ShippingAddressLine2 = \"#{ encodeURIComponent @model.line2 }\""

    if @model.city
      query += " AND ShippingAddressCity = \"#{ encodeURIComponent @model.city }\""

    if @model.state
      query += " AND ShippingAddressState = \"#{ encodeURIComponent @model.state }\""

    if @model.postal
      query += " AND ShippingAddressPostalCode = \"#{ encodeURIComponent @model.postal }\""

    if @model.country != '_any'
      query += " AND ShippingAddressCountryCode = \"#{ encodeURIComponent @model.country }\""

    return query

UserFilterPane.register()

module.exports = UserFilterPane
