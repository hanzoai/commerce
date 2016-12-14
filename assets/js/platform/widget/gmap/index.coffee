riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
View = crowdcontrol.view.View

class Gmap extends View
  tag: 'gmap'
  addressField: 'address'

  events:
    "#{Events.Input.Set}": (name)->
      if name == (@addressField + '.line1')      ||
      name == (@addressField + '.line2')         ||
      name == (@addressField + '.city')          ||
      name == (@addressField + '.state')         ||
      name == (@addressField + '.postalCode')    ||
      name == (@addressField + '.country')

        @refresh()

    "#{Events.Form.Prefill}": (model)->
      @model = model
      @refresh()

  refresh: ()->
    if @model?[@addressField]?
      address = @model[@addressField].line1 + ' ' +
        ((@model[@addressField].line2 + ' ') if @model[@addressField].line2) +
        @model[@addressField].city + ' ' +
        @model[@addressField].state + ' ' +
        @model[@addressField].postalCode + ' ' +
        @model[@addressField].country

    if address != @lastAddress
      @lastAddress = address
      GMaps.geocode
        address: address
        callback: (results, status) =>
          if status == 'OK'
            if !@map?
              @map = new GMaps
                div: @root
                lat: 21.3280681
                lng: -157.798970564
                zoom: 12

            latlng = results[0].geometry.location
            @map.removeMarkers()
            @map.setCenter latlng.lat(), latlng.lng()
            @map.addMarker
              lat: latlng.lat()
              lng: latlng.lng()

  js: (opts)->
    super()

    @addressField = opts.addressfield ? @addressField

    $(@root).addClass('map')

    @on 'update', ()=>
      @refresh()

Gmap.register()

module.exports = Gmap
