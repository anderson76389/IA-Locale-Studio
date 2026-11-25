#include <jni.h>
#include <atomic>
#include <mutex>
#include <memory>
#include <stdexcept>
#include <string>
#include <vector>
#include <algorithm>
#include "llama.h"
static std::atomic<bool> stopped{false};
static std::once_flag initialized;
static std::mutex execution;
static void fail(JNIEnv *env, const char *msg) { env->ThrowNew(env->FindClass("java/lang/IllegalStateException"),msg); }
extern "C" JNIEXPORT void JNICALL Java_fr_ialocale_mobile_MainActivity_resetStop(JNIEnv*,jclass){stopped=false;}
extern "C" JNIEXPORT void JNICALL Java_fr_ialocale_mobile_MainActivity_stopNative(JNIEnv*,jclass){stopped=true;}
extern "C" JNIEXPORT jbyteArray JNICALL Java_fr_ialocale_mobile_MainActivity_generateNative(JNIEnv *env,jclass,jstring path,jbyteArray input,jint max_tokens,jint threads) {
  std::lock_guard<std::mutex> lock(execution);
  try {
    if(stopped) throw std::runtime_error("Generation annulee");
    std::call_once(initialized, []{llama_backend_init();});
    const char *raw=env->GetStringUTFChars(path,nullptr);
    if(!raw) return nullptr;
    std::string file(raw); env->ReleaseStringUTFChars(path,raw);
    std::string prompt(env->GetArrayLength(input),'\0');
    env->GetByteArrayRegion(input,0,prompt.size(),reinterpret_cast<jbyte*>(prompt.data()));
    auto mp=llama_model_default_params(); mp.n_gpu_layers=0; mp.use_mmap=true;
    std::unique_ptr<llama_model,decltype(&llama_model_free)> model(llama_model_load_from_file(file.c_str(),mp),llama_model_free);
    if(!model) throw std::runtime_error("Modele GGUF incompatible ou memoire insuffisante");
    if(stopped) throw std::runtime_error("Generation annulee");
    const char *tmpl=llama_model_chat_template(model.get(),nullptr);
    if(tmpl){
      llama_chat_message msg{"user",prompt.c_str()};
      int n=llama_chat_apply_template(tmpl,&msg,1,true,nullptr,0);
      if(n>0 && n<1000000){std::vector<char> b(n+1); int actual=llama_chat_apply_template(tmpl,&msg,1,true,b.data(),b.size()); if(actual>0 && actual<=n) prompt.assign(b.data(),actual);}
    }
    const auto *vocab=llama_model_get_vocab(model.get());
    int n=llama_tokenize(vocab,prompt.data(),prompt.size(),nullptr,0,true,true);
    if(n>=0 || n< -1800) throw std::runtime_error("Prompt vide ou trop long (contexte 2048)");
    std::vector<llama_token> tokens(-n);
    if(llama_tokenize(vocab,prompt.data(),prompt.size(),tokens.data(),tokens.size(),true,true)!=-n) throw std::runtime_error("Tokenisation impossible");
    auto cp=llama_context_default_params(); cp.n_ctx=2048; cp.n_batch=2048; cp.n_ubatch=256; cp.n_threads=std::clamp((int)threads,1,8); cp.n_threads_batch=cp.n_threads;
    std::unique_ptr<llama_context,decltype(&llama_free)> ctx(llama_init_from_model(model.get(),cp),llama_free);
    if(!ctx) throw std::runtime_error("Contexte non disponible");
    llama_set_abort_callback(ctx.get(),[](void*)->bool{return stopped.load();},nullptr);
    if(llama_decode(ctx.get(),llama_batch_get_one(tokens.data(),tokens.size()))!=0) throw std::runtime_error(stopped?"Generation annulee":"Echec du calcul initial");
    std::unique_ptr<llama_sampler,decltype(&llama_sampler_free)> sampler(llama_sampler_init_greedy(),llama_sampler_free);
    if(!sampler) throw std::runtime_error("Echantillonneur non disponible");
    std::string out;
    int limit=std::min({(int)max_tokens,512,2048-(int)tokens.size()});
    for(int i=0;i<limit && !stopped;i++){
      llama_token token=llama_sampler_sample(sampler.get(),ctx.get(),-1);
      if(llama_vocab_is_eog(vocab,token)) break;
      char small[256]; int len=llama_token_to_piece(vocab,token,small,sizeof(small),0,false);
      if(len>=0) out.append(small,len);
      else {std::vector<char> buf(-len); len=llama_token_to_piece(vocab,token,buf.data(),buf.size(),0,false); if(len<0) throw std::runtime_error("Token invalide"); out.append(buf.data(),len);}
      if(i+1<limit && !stopped && llama_decode(ctx.get(),llama_batch_get_one(&token,1))!=0) throw std::runtime_error(stopped?"Generation annulee":"Echec de la generation");
    }
    if(stopped) throw std::runtime_error("Generation annulee");
    if(out.empty()) throw std::runtime_error("Le modele n'a produit aucun texte");
    jbyteArray result=env->NewByteArray(out.size()); if(result) env->SetByteArrayRegion(result,0,out.size(),reinterpret_cast<const jbyte*>(out.data())); return result;
  } catch(const std::exception &e){fail(env,e.what());return nullptr;}
}
