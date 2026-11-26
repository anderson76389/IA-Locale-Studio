package fr.ialocale.mobile;
import android.app.Activity;
import android.os.Bundle;
import android.content.Intent;
import android.content.Context;
import android.graphics.Color;
import android.net.*;
import android.view.View;
import android.widget.*;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;
public final class MainActivity extends Activity {
    private static final int PICK=40;
    private final ExecutorService worker=Executors.newSingleThreadExecutor();
    private final AtomicBoolean cancelled=new AtomicBoolean();
    private boolean nativeReady=false,busy=false, code=false;
    private TextView state, modelLabel, output, connection;
    private EditText prompt;
    private Button run, stop, pick, download, textTab, codeTab;
    private File model;
    private static native byte[] generateNative(String path, byte[] input, int maxTokens, int threads);
    private static native void resetStop();
    private static native void stopNative();
    private int dp(int x){return (int)(getResources().getDisplayMetrics().density*x+.5f);}
    private TextView label(String s,int size){TextView t=new TextView(this);t.setText(s);t.setTextSize(size);t.setTextColor(Color.WHITE);t.setPadding(0,dp(8),0,dp(8));return t;}
    private Button button(String s,View.OnClickListener listener){Button b=new Button(this);b.setText(s);b.setAllCaps(false);b.setOnClickListener(listener);return b;}
    @Override public void onCreate(Bundle b){super.onCreate(b);
        try{System.loadLibrary("ialocale");nativeReady=true;}catch(LinkageError e){nativeReady=false;}
        String previous=getPreferences(0).getString("model","");if(!previous.isEmpty()){File f=new File(getFilesDir(),"models/"+new File(previous).getName());if(f.isFile()) model=f;}
        ScrollView scroll=new ScrollView(this);scroll.setFillViewport(true);
        LinearLayout root=new LinearLayout(this);root.setOrientation(LinearLayout.VERTICAL);root.setPadding(dp(20),dp(32),dp(20),dp(24));root.setBackgroundColor(0xff111521);scroll.addView(root);
        root.addView(label("IA Locale Studio",28));root.addView(label("Android • Calcul local CPU",15));connection=label("",14);root.addView(connection);
        LinearLayout tabs=new LinearLayout(this);textTab=button("Texte",v->select(false));codeTab=button("Code",v->select(true));tabs.addView(textTab,new LinearLayout.LayoutParams(0,-2,1));tabs.addView(codeTab,new LinearLayout.LayoutParams(0,-2,1));root.addView(tabs);
        root.addView(label("Image · Vidéo · Musique : non disponibles dans cette édition native.",14));
        modelLabel=label("",14);root.addView(modelLabel);pick=button("Importer un modèle GGUF",v->{if(busy)return;Intent i=new Intent(Intent.ACTION_OPEN_DOCUMENT);i.setType("*/*");i.addCategory(Intent.CATEGORY_OPENABLE);startActivityForResult(i,PICK);});root.addView(pick);
        download=button("Télécharger un modèle sur Hugging Face",v->{refreshNetwork();if(!online()){state.setText("Vous êtes hors ligne. Connectez-vous pour télécharger un modèle. Les modèles installés restent utilisables.");return;}try{startActivity(new Intent(Intent.ACTION_VIEW,Uri.parse("https://huggingface.co/models?library=gguf")));}catch(Exception e){state.setText("Aucun navigateur disponible.");}});root.addView(download);
        root.addView(label("Demande",16));prompt=new EditText(this);prompt.setHint("Écrire une demande…");prompt.setTextColor(Color.WHITE);prompt.setHintTextColor(0xffa5aabb);prompt.setMinLines(4);root.addView(prompt);
        run=button("Générer localement",v->start());root.addView(run);stop=button("Arrêter",v->{cancelled.set(true);if(nativeReady)stopNative();state.setText("Arrêt demandé…");});root.addView(stop);state=label("Prêt",14);root.addView(state);output=label("",16);output.setTextIsSelectable(true);root.addView(output);setContentView(scroll);select(false);refresh();refreshNetwork();
    }
    private void select(boolean c){code=c;textTab.setEnabled(!busy&&c);codeTab.setEnabled(!busy&&!c);}
    private boolean online(){ConnectivityManager cm=(ConnectivityManager)getSystemService(Context.CONNECTIVITY_SERVICE);NetworkCapabilities caps=cm.getNetworkCapabilities(cm.getActiveNetwork());return caps!=null&&caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED);}
    private void refreshNetwork(){connection.setText(online()?"En ligne · calcul toujours local":"Hors ligne · modèles installés utilisables");}
    @Override protected void onResume(){super.onResume();if(connection!=null)refreshNetwork();}
    private void refresh(){modelLabel.setText(model==null?"Aucun modèle installé":"Modèle installé : "+(model.length()/1048576)+" Mio");run.setEnabled(nativeReady&&!busy&&model!=null);pick.setEnabled(!busy);download.setEnabled(!busy);stop.setEnabled(busy);select(code);if(!nativeReady)state.setText("Moteur natif non chargé. La génération est indisponible.");}
    @Override protected void onActivityResult(int req,int result,Intent data){super.onActivityResult(req,result,data);if(req!=PICK||result!=RESULT_OK||data==null||data.getData()==null||busy)return;Uri uri=data.getData();busy=true;cancelled.set(false);refresh();state.setText("Import local en cours…");worker.submit(()->{try{InputStream in=getContentResolver().openInputStream(uri);if(in==null)throw new IOException("Fichier inaccessible");File dest=ModelFiles.copy(in,new File(getFilesDir(),"models"),cancelled::get);runOnUiThread(()->{if(isDestroyed())return;model=dest;getPreferences(0).edit().putString("model",dest.getName()).apply();finishJob("Modèle importé.");});}catch(Exception e){runOnUiThread(()->finishJob("Import : "+e.getMessage()));}});}
    private void start(){if(busy||!nativeReady||model==null)return;String q=prompt.getText().toString().trim();if(q.isEmpty()){state.setText("Écris une demande.");return;}if(q.length()>6000){state.setText("Demande trop longue pour ce profil.");return;}File selected=model;String request=code?"Réponds avec du code et des explications utiles :\n"+q:q;busy=true;cancelled.set(false);resetStop();refresh();output.setText("");state.setText("Calcul local… (aucune requête Internet)");worker.submit(()->{try{byte[] result=generateNative(selected.getAbsolutePath(),request.getBytes(StandardCharsets.UTF_8),384,Math.min(4,Runtime.getRuntime().availableProcessors()));String answer=new String(result,StandardCharsets.UTF_8);runOnUiThread(()->{if(isDestroyed())return;output.setText(answer);finishJob("Génération terminée · modèle déchargé.");});}catch(Exception e){runOnUiThread(()->finishJob("Génération : "+e.getMessage()));}});}
    private void finishJob(String s){if(isDestroyed())return;busy=false;state.setText(s);refresh();}
    @Override protected void onDestroy(){cancelled.set(true);if(nativeReady)stopNative();worker.shutdownNow();super.onDestroy();}
}
