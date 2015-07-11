crowdcontrol = require 'crowdcontrol'
dropzone = require 'dropzone'

class DropZone extends crowdcontrol.view.View
  tag: 'dropzone'
  path: 'https://www.googleapis.com/upload/storage/v1/b'
  bucket: 'unknown'
  queryParams: '?uploadType=media&predefinedAcl'
  html: require './template.html'
  js: (opts)->
    @path = opts.path if opts.path?
    @bucket = opts.bucket if opts.bucket?

    requestAnimationFrame ()=>
      @dz = new dropzone document.body,
        url: @path + '/' + @bucket + '/o' + @queryParams + '&name='
        thumbnailWidth: 80
        thumbnailHeight: 80
        parallelUploads: 20
        previewTemplate: require './preview-template.html'
        previewsContainer: '.previews'
        clickable: '.add-image'

DropZone.register()

