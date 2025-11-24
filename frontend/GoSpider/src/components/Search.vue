<template>
  <div class="container my-5">
    <div class="col-12 col-md-8 col-lg-6 mx-auto">
      <div class="d-flex flex-column align-items-center">
        <a href="#" class="mb-2">
          <img 
            :src="logoSource" 
            alt="logo" 
            loading="lazy" 
            class="main-logo img-fluid"
          >
        </a>
        <h1 class="mb-2">GoSpider</h1>
        <input 
          type="search" 
          placeholder="Search for products, documentation, or services..." 
          class="form-control rounded-pill search-input mt-2"
        >
      </div>
    </div>
  </div>
</template>

<script>
import logo from "@/assets/logo.webp"
export default{
  name:"Search",
  data(){
    return {
      logoSource:logo
    }
  },
  mounted(){

  },
  methods:{
    
  }
}

</script>  

<style scoped>
.main-logo{
  width:auto;
  height:12rem;
}
.search-input{
  padding-left:1.5rem;
  padding-right:1.5rem;
}
</style>
